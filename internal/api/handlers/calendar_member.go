package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jaysongiroux/go-scheduler/internal/api/web"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/calendar_member"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
	"github.com/jaysongiroux/go-scheduler/internal/workers"
)

// InviteMembersRequest represents the request body for inviting members
type InviteMembersRequest struct {
	AccountID  string   `json:"account_id"`  // inviter's account ID
	AccountIDs []string `json:"account_ids"` // invitees
	Role       string   `json:"role"`        // read or write
}

// UpdateMemberRequest represents the request body for updating a member
type UpdateMemberRequest struct {
	Status *string `json:"status,omitempty"` // pending or confirmed
	Role   *string `json:"role,omitempty"`   // read or write
}

func (h *Handler) InviteCalendarMembers(w http.ResponseWriter, r *http.Request) {
	calendarUID, err := uuid.Parse(r.PathValue("calendar_uid"))
	if err != nil {
		logger.Warn("Failed to parse calendar UID: %v", err)
		web.RespondError(w, http.StatusBadRequest, "Invalid calendar UID", err.Error())
		return
	}

	var req InviteMembersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Warn("Failed to decode request body: %v", err)
		web.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Get the inviter's account ID from request body
	inviterAccountID := req.AccountID
	if inviterAccountID == "" {
		web.RespondError(w, http.StatusBadRequest, "account_id is required in request body")
		return
	}

	// Verify the calendar exists and the user is the owner
	cal, err := h.CalendarRepo.GetCalendar(r.Context(), calendarUID)
	if err != nil {
		logger.Warn("Calendar not found: %v", err)
		web.RespondError(w, http.StatusNotFound, "Calendar not found", err.Error())
		return
	}

	if cal.AccountID != inviterAccountID {
		web.RespondError(w, http.StatusForbidden, "Only the calendar owner can invite members")
		return
	}

	if len(req.AccountIDs) == 0 {
		web.RespondError(w, http.StatusBadRequest, "account_ids is required and must not be empty")
		return
	}

	// Validate role
	role := req.Role
	if role == "" {
		role = "write" // default to write
	}
	if role != "read" && role != "write" {
		web.RespondError(w, http.StatusBadRequest, "role must be 'read' or 'write'")
		return
	}

	now := time.Now().Unix()
	members := make([]*calendar_member.CalendarMember, 0, len(req.AccountIDs))

	for _, accountID := range req.AccountIDs {
		// Don't allow inviting the owner
		if accountID == cal.AccountID {
			continue
		}

		member := &calendar_member.CalendarMember{
			AccountID:   accountID,
			CalendarUID: calendarUID,
			Status:      "pending",
			Role:        role,
			InvitedBy:   inviterAccountID,
			InvitedAtTs: now,
			UpdatedTs:   now,
		}
		members = append(members, member)
	}

	if len(members) == 0 {
		web.RespondError(w, http.StatusBadRequest, "No valid members to invite")
		return
	}

	if err := h.CalendarMemberRepo.CreateCalendarMembers(r.Context(), members); err != nil {
		logger.Error("Failed to create calendar members: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to invite members", err.Error())
		return
	}

	web.RespondJSON(w, http.StatusCreated, map[string]interface{}{
		"invited_count": len(members),
		"members":       members,
	})

	// Queue webhook deliveries for each invite
	for _, member := range members {
		if err := h.WebhookDispatcher.QueueDelivery(
			r.Context(),
			workers.MemberInvited,
			member,
			nil,
		); err != nil {
			logger.Error("Failed to queue webhook for member invite: %v", err)
		}
	}
}

func (h *Handler) GetCalendarMembers(w http.ResponseWriter, r *http.Request) {
	calendarUID, err := uuid.Parse(r.PathValue("calendar_uid"))
	if err != nil {
		logger.Warn("Failed to parse calendar UID: %v", err)
		web.RespondError(w, http.StatusBadRequest, "Invalid calendar UID", err.Error())
		return
	}

	// Get the requesting user's account ID
	accountID := r.URL.Query().Get("account_id")
	if accountID == "" {
		web.RespondError(w, http.StatusBadRequest, "account_id query parameter is required")
		return
	}

	// Verify the calendar exists
	_, err = h.CalendarRepo.GetCalendar(r.Context(), calendarUID)
	if err != nil {
		logger.Warn("Calendar not found: %v", err)
		web.RespondError(w, http.StatusNotFound, "Calendar not found", err.Error())
		return
	}

	// Verify the user is the owner or a member
	isMember, err := h.CalendarMemberRepo.IsMemberOfCalendar(r.Context(), accountID, calendarUID)
	if err != nil {
		logger.Error("Failed to check membership: %v", err)
		web.RespondError(
			w,
			http.StatusInternalServerError,
			"Failed to check membership",
			err.Error(),
		)
		return
	}

	if !isMember {
		web.RespondError(w, http.StatusForbidden, "You do not have access to this calendar")
		return
	}

	members, err := h.CalendarMemberRepo.GetCalendarMembers(r.Context(), calendarUID)
	if err != nil {
		logger.Error("Failed to get calendar members: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to get members", err.Error())
		return
	}

	web.RespondJSON(w, http.StatusOK, members)
}

func (h *Handler) UpdateCalendarMember(w http.ResponseWriter, r *http.Request) {
	calendarUID, err := uuid.Parse(r.PathValue("calendar_uid"))
	if err != nil {
		logger.Warn("Failed to parse calendar UID: %v", err)
		web.RespondError(w, http.StatusBadRequest, "Invalid calendar UID", err.Error())
		return
	}

	memberAccountID := r.PathValue("account_id")
	if memberAccountID == "" {
		web.RespondError(w, http.StatusBadRequest, "account_id path parameter is required")
		return
	}

	// Get the requesting user's account ID
	requestingAccountID := r.URL.Query().Get("account_id")
	if requestingAccountID == "" {
		web.RespondError(w, http.StatusBadRequest, "account_id query parameter is required")
		return
	}

	// Verify the calendar exists
	cal, err := h.CalendarRepo.GetCalendar(r.Context(), calendarUID)
	if err != nil {
		logger.Warn("Calendar not found: %v", err)
		web.RespondError(w, http.StatusNotFound, "Calendar not found", err.Error())
		return
	}

	var req UpdateMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Warn("Failed to decode request body: %v", err)
		web.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	now := time.Now().Unix()
	isOwner := cal.AccountID == requestingAccountID

	// Handle status update (accept/reject)
	if req.Status != nil {
		status := *req.Status
		if status != "pending" && status != "confirmed" {
			web.RespondError(w, http.StatusBadRequest, "status must be 'pending' or 'confirmed'")
			return
		}

		// Only the member themselves can accept/reject
		if requestingAccountID != memberAccountID {
			web.RespondError(
				w,
				http.StatusForbidden,
				"Only the member can accept or reject the invitation",
			)
			return
		}

		// If rejecting (setting to pending after being invited), delete the member
		if status == "pending" {
			if err := h.CalendarMemberRepo.DeleteCalendarMember(r.Context(), memberAccountID, calendarUID); err != nil {
				logger.Error("Failed to delete calendar member: %v", err)
				web.RespondError(
					w,
					http.StatusInternalServerError,
					"Failed to reject invitation",
					err.Error(),
				)
				return
			}

			web.ResponseSuccess(w)

			// Queue webhook for rejection
			if err := h.WebhookDispatcher.QueueDelivery(
				r.Context(),
				workers.MemberRejected,
				map[string]interface{}{
					"account_id":   memberAccountID,
					"calendar_uid": calendarUID.String(),
				},
				nil,
			); err != nil {
				logger.Error("Failed to queue webhook for member rejection: %v", err)
			}
			return
		}

		// Accept the invitation
		if err := h.CalendarMemberRepo.UpdateMemberStatus(r.Context(), memberAccountID, calendarUID, status, now); err != nil {
			logger.Error("Failed to update member status: %v", err)
			web.RespondError(
				w,
				http.StatusInternalServerError,
				"Failed to update status",
				err.Error(),
			)
			return
		}

		member, err := h.CalendarMemberRepo.GetCalendarMember(
			r.Context(),
			memberAccountID,
			calendarUID,
		)
		if err != nil {
			logger.Error("Failed to get updated member: %v", err)
			web.RespondError(
				w,
				http.StatusInternalServerError,
				"Failed to get updated member",
				err.Error(),
			)
			return
		}

		web.RespondJSON(w, http.StatusOK, member)

		// Queue webhook for acceptance
		if err := h.WebhookDispatcher.QueueDelivery(
			r.Context(),
			workers.MemberAccepted,
			member,
			nil,
		); err != nil {
			logger.Error("Failed to queue webhook for member acceptance: %v", err)
		}
		return
	}

	// Handle role update
	if req.Role != nil {
		role := *req.Role
		if role != "read" && role != "write" {
			web.RespondError(w, http.StatusBadRequest, "role must be 'read' or 'write'")
			return
		}

		// Only the owner can change roles
		if !isOwner {
			web.RespondError(
				w,
				http.StatusForbidden,
				"Only the calendar owner can change member roles",
			)
			return
		}

		if err := h.CalendarMemberRepo.UpdateMemberRole(r.Context(), memberAccountID, calendarUID, role, now); err != nil {
			logger.Error("Failed to update member role: %v", err)
			web.RespondError(
				w,
				http.StatusInternalServerError,
				"Failed to update role",
				err.Error(),
			)
			return
		}

		member, err := h.CalendarMemberRepo.GetCalendarMember(
			r.Context(),
			memberAccountID,
			calendarUID,
		)
		if err != nil {
			logger.Error("Failed to get updated member: %v", err)
			web.RespondError(
				w,
				http.StatusInternalServerError,
				"Failed to get updated member",
				err.Error(),
			)
			return
		}

		web.RespondJSON(w, http.StatusOK, member)
		return
	}

	web.RespondError(w, http.StatusBadRequest, "Either status or role must be provided")
}

func (h *Handler) RemoveCalendarMember(w http.ResponseWriter, r *http.Request) {
	calendarUID, err := uuid.Parse(r.PathValue("calendar_uid"))
	if err != nil {
		logger.Warn("Failed to parse calendar UID: %v", err)
		web.RespondError(w, http.StatusBadRequest, "Invalid calendar UID", err.Error())
		return
	}

	memberAccountID := r.PathValue("account_id")
	if memberAccountID == "" {
		web.RespondError(w, http.StatusBadRequest, "account_id path parameter is required")
		return
	}

	// Get the requesting user's account ID
	requestingAccountID := r.URL.Query().Get("account_id")
	if requestingAccountID == "" {
		web.RespondError(w, http.StatusBadRequest, "account_id query parameter is required")
		return
	}

	// Verify the calendar exists
	cal, err := h.CalendarRepo.GetCalendar(r.Context(), calendarUID)
	if err != nil {
		logger.Warn("Calendar not found: %v", err)
		web.RespondError(w, http.StatusNotFound, "Calendar not found", err.Error())
		return
	}

	isOwner := cal.AccountID == requestingAccountID
	isSelf := requestingAccountID == memberAccountID

	// Only the owner can remove others, or members can remove themselves
	if !isOwner && !isSelf {
		web.RespondError(
			w,
			http.StatusForbidden,
			"You can only remove yourself or you must be the calendar owner",
		)
		return
	}

	if err := h.CalendarMemberRepo.DeleteCalendarMember(r.Context(), memberAccountID, calendarUID); err != nil {
		logger.Error("Failed to delete calendar member: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to remove member", err.Error())
		return
	}

	web.ResponseSuccess(w)

	// Queue webhook for member removal
	if err := h.WebhookDispatcher.QueueDelivery(
		r.Context(),
		workers.MemberRemoved,
		map[string]interface{}{
			"account_id":   memberAccountID,
			"calendar_uid": calendarUID.String(),
			"removed_by":   requestingAccountID,
		},
		nil,
	); err != nil {
		logger.Error("Failed to queue webhook for member removal: %v", err)
	}
}
