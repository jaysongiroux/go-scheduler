import { afterAll, beforeAll, describe, expect, test } from "vitest";
import {
  AccountObject,
  CalendarObject,
  ErrorObject,
  GenericPagedResponse,
  SuccessObject,
} from "../helpers/types";
import { createAccount, deleteAccount } from "../helpers/account";
import {
  createCalendar,
  deleteCalendar,
  getCalendar,
  getUserCalendars,
  updateCalendar,
} from "../helpers/calendar";

describe("Single Event API", () => {
  const accounts: string[] = [];
  let calendarUid: string;
  let eventUid: string;

  afterAll(async () => {
    // deleting accounts will cascade to delete calendars and events
    for (const account of accounts) {
      const response = (await deleteAccount({
        accountId: account,
      })) as SuccessObject;
      expect(response.success).toBe(true);
    }
  });

  beforeAll(async () => {
    const accountUUID = crypto.randomUUID();
    const response = (await createAccount({
      accountId: accountUUID,
    })) as AccountObject;
    expect(response.account_id).toBe(accountUUID);
    accounts.push(response.account_id);
  });

  test("Should create a calendar", async () => {
    const response = (await createCalendar({
      accountId: accounts[0],
    })) as CalendarObject;
    expect(response.calendar_uid).toBeDefined();
    expect(response.account_id).toBe(accounts[0]);
    calendarUid = response.calendar_uid;
  });

  test("Should get a calendar", async () => {
    const response = (await getCalendar({
      calendarUid: calendarUid,
    })) as CalendarObject;
    expect(response.calendar_uid).toBe(calendarUid);
    expect(response.account_id).toBe(accounts[0]);
  });

  test("Should update a calendar", async () => {
    const response = (await updateCalendar({
      calendarUid: calendarUid,
      settings: { test: "value" },
      metadata: { test: "calendar" },
    })) as CalendarObject;
    expect(response.calendar_uid).toBe(calendarUid);
    expect(response.account_id).toBe(accounts[0]);
    expect(response.settings).toEqual({ test: "value" });
    expect(response.metadata).toEqual({ test: "calendar" });
  });

  test("Should get user calendars", async () => {
    const response = (await getUserCalendars({
      accountId: accounts[0],
    })) as GenericPagedResponse<CalendarObject>;
    expect(response.count).toBe(1);
    expect(response.data[0].calendar_uid).toBe(calendarUid);
    expect(response.data[0].account_id).toBe(accounts[0]);
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
