import { SchedulerClient } from "./src";

async function main() {
  const client = new SchedulerClient({
    baseURL: "http://localhost:8080",
    timeout: 30000,
  });

  try {
    // Check health
    const health = await client.healthCheck();
    console.log("Health check:", health);

    // Create a calendar
    const calendar = await client.calendars.create({
      account_id: "user123",
      name: "My Calendar",
      description: "Personal calendar",
      timezone: "America/New_York",
      metadata: { color: "blue" },
    });
    console.log("Created calendar:", calendar);

    // Create an event
    const now = Math.floor(Date.now() / 1000);
    const event = await client.events.create({
      calendar_uid: calendar.calendar_uid,
      account_id: "user123",
      start_ts: now + 3600,
      end_ts: now + 7200,
      timezone: "America/New_York",
      metadata: {
        title: "Team Meeting",
        description: "Quarterly planning",
      },
    });
    console.log("Created event:", event);

    // Create a reminder (30 minutes before)
    const reminder = await client.reminders.create(event.event_uid, {
      offset_seconds: -1800,
      account_id: "user123",
      metadata: { method: "email" },
      scope: "single",
    });
    console.log("Created reminder:", reminder);

    // List events
    const events = await client.events.getCalendarEvents({
      calendar_uids: [calendar.calendar_uid],
      start_ts: now,
      end_ts: now + 86400 * 7,
    });
    console.log("Events:", events);
  } catch (error) {
    console.error("Error:", error);
  }
}

main();
