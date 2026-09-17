Timetable should be read easily by the program and then transformed into an .ics file.

```text
go run . -input timetable_2026-09-12.pdf -output timetable.ics
```

## Telegram bot

Run the long-polling bot with a Telegram token:

```text
set TELEGRAM_BOT_TOKEN=your-token
go run . -bot
```

Schedules are kept in memory per Telegram chat. Sending a new PDF replaces the
previous schedule. The bot uses Europe/Vilnius time and supports:

- `/ics` - download the current `.ics` calendar;
- `/remind MINUTES` or `/remind off` - configure lecture reminders;
- `/tomorrow HH:MM` - receive the next day's lectures at that local time;
- `/help` - show the available commands.

The bot uses the semester dates found in the uploaded PDF and loses schedules
when the process stops.