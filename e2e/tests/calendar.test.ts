import { describe, expect, test } from "vitest";
import {
  CalendarObject,
  ErrorObject,
  GenericPagedResponse,
  SuccessObject,
} from "../helpers/types";
import {
  createCalendar,
  deleteCalendar,
  getCalendar,
  getUserCalendars,
  updateCalendar,
} from "../helpers/calendar";

describe("Single Event API", () => {
  let calendarUid: string;
  let accountId: string = crypto.randomUUID();

  test("Should create a calendar", async () => {
    const response = (await createCalendar({
      accountId: accountId,
    })) as CalendarObject;
    expect(response.calendar_uid).toBeDefined();
    expect(response.account_id).toBe(accountId);
    calendarUid = response.calendar_uid;
  });

  test("Should get a calendar", async () => {
    const response = (await getCalendar({
      calendarUid: calendarUid,
    })) as CalendarObject;
    expect(response.calendar_uid).toBe(calendarUid);
    expect(response.account_id).toBe(accountId);
  });

  test("Should update a calendar", async () => {
    const response = (await updateCalendar({
      calendarUid: calendarUid,
      settings: { test: "value" },
      metadata: { test: "calendar" },
    })) as CalendarObject;
    expect(response.calendar_uid).toBe(calendarUid);
    expect(response.account_id).toBe(accountId);
    expect(response.settings).toEqual({ test: "value" });
    expect(response.metadata).toEqual({ test: "calendar" });
  });

  test("Should get user calendars", async () => {
    const response = (await getUserCalendars({
      accountId: accountId,
    })) as GenericPagedResponse<CalendarObject>;
    expect(response.count).toBe(1);
    expect(response.data[0].calendar_uid).toBe(calendarUid);
    expect(response.data[0].account_id).toBe(accountId);
    expect(response.data[0].settings).toEqual({ test: "value" });
    expect(response.data[0].metadata).toEqual({ test: "calendar" });
  });

  test("Should delete a calendar", async () => {
    const response = (await deleteCalendar({
      calendarUid: calendarUid,
    })) as SuccessObject;
    expect(response.success).toBe(true);

    const response2 = (await getCalendar({
      calendarUid: calendarUid,
    })) as ErrorObject;
    expect(response2.error).toBe("Calendar not found");
  });
});
