package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coregx/gxpdf"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/megazvezda/happiness/models"
)

const botTimezone = "Europe/Vilnius"

type userSchedule struct {
	ChatID         int64
	Schedule       Schedule
	ReminderBefore int
	TomorrowAt     string
	LastReminder   string
	LastTomorrow   string
}

type botStore struct {
	mu    sync.RWMutex
	users map[int64]*userSchedule
}

func runBot() error {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	location, err := time.LoadLocation(botTimezone)
	if err != nil {
		return fmt.Errorf("load bot timezone: %w", err)
	}
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return fmt.Errorf("create Telegram bot: %w", err)
	}
	updates := bot.GetUpdatesChan(tgbotapi.UpdateConfig{Timeout: 60})

	store := &botStore{users: make(map[int64]*userSchedule)}
	go reminderLoop(bot, store, location)
	for update := range updates {
		if update.Message != nil {
			handleMessage(bot, store, update.Message)
		}
	}
	return nil
}

func handleMessage(bot *tgbotapi.BotAPI, store *botStore, message *tgbotapi.Message) {
	if message.Document != nil {
		if err := handlePDF(bot, store, message); err != nil {
			sendText(bot, message.Chat.ID, "I could not process that PDF: "+err.Error())
		}
		return
	}
	if !message.IsCommand() {
		sendText(bot, message.Chat.ID, "Send a timetable PDF, or use /help for commands.")
		return
	}

	switch message.Command() {
	case "start", "help":
		sendText(bot, message.Chat.ID, "/remind MINUTES enables reminders before lectures; /remind off disables them.\n/tomorrow HH:MM sends tomorrow's lectures at that local time.\n/ics sends your current calendar.\nSend a new PDF to replace your schedule.")
	case "remind":
		handleRemind(bot, store, message)
	case "tomorrow":
		handleTomorrow(bot, store, message)
	case "ics":
		handleICS(bot, store, message)
	default:
		sendText(bot, message.Chat.ID, "Unknown command. Use /help.")
	}
}

func handlePDF(bot *tgbotapi.BotAPI, store *botStore, message *tgbotapi.Message) error {
	file, err := bot.GetFile(tgbotapi.FileConfig{FileID: message.Document.FileID})
	if err != nil {
		return fmt.Errorf("download metadata: %w", err)
	}
	response, err := http.Get(file.Link(bot.Token))
	if err != nil {
		return fmt.Errorf("download PDF: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download PDF returned HTTP %s", response.Status)
	}

	tempFile, err := os.CreateTemp("", "happiness-*.pdf")
	if err != nil {
		return fmt.Errorf("create temporary PDF: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	if _, err := io.Copy(tempFile, response.Body); err != nil {
		tempFile.Close()
		return fmt.Errorf("save PDF: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temporary PDF: %w", err)
	}

	doc, err := gxpdf.Open(tempPath)
	if err != nil {
		return fmt.Errorf("open PDF: %w", err)
	}
	schedule, parseErr := parseSchedule(doc.ExtractTables())
	closeErr := doc.Close()
	if parseErr != nil {
		return parseErr
	}
	if closeErr != nil {
		return fmt.Errorf("close PDF: %w", closeErr)
	}
	if schedule.StartDate.IsZero() || schedule.EndDate.IsZero() {
		return fmt.Errorf("could not find semester dates in the PDF")
	}

	store.mu.Lock()
	key := message.Chat.ID
	if message.From != nil {
		key = message.From.ID
	}
	current := store.users[key]
	if current == nil {
		current = &userSchedule{ChatID: message.Chat.ID}
		store.users[key] = current
	}
	current.Schedule = schedule
	current.LastReminder = ""
	current.LastTomorrow = ""
	store.mu.Unlock()
	sendText(bot, message.Chat.ID, "Schedule saved. Use /ics to download it, /remind MINUTES for lecture reminders, or /tomorrow HH:MM.")
	return nil
}

func handleRemind(bot *tgbotapi.BotAPI, store *botStore, message *tgbotapi.Message) {
	args := strings.Fields(message.CommandArguments())
	if len(args) != 1 {
		sendText(bot, message.Chat.ID, "Usage: /remind MINUTES or /remind off")
		return
	}
	store.mu.Lock()
	key := message.Chat.ID
	if message.From != nil {
		key = message.From.ID
	}
	user := store.users[key]
	if user == nil {
		store.mu.Unlock()
		sendText(bot, message.Chat.ID, "Send your timetable PDF first.")
		return
	}
	if args[0] == "off" {
		user.ReminderBefore = 0
	} else {
		minutes, err := strconv.Atoi(args[0])
		if err != nil || minutes < 1 || minutes > 1440 {
			store.mu.Unlock()
			sendText(bot, message.Chat.ID, "Minutes must be between 1 and 1440.")
			return
		}
		user.ReminderBefore = minutes
	}
	value := user.ReminderBefore
	store.mu.Unlock()
	if value == 0 {
		sendText(bot, message.Chat.ID, "Lecture reminders disabled.")
	} else {
		sendText(bot, message.Chat.ID, fmt.Sprintf("I will remind you %d minutes before each lecture.", value))
	}
}

func handleTomorrow(bot *tgbotapi.BotAPI, store *botStore, message *tgbotapi.Message) {
	args := strings.Fields(message.CommandArguments())
	if len(args) != 1 {
		sendText(bot, message.Chat.ID, "Usage: /tomorrow HH:MM")
		return
	}
	if _, err := time.Parse("15:04", args[0]); err != nil {
		sendText(bot, message.Chat.ID, "Time must use HH:MM.")
		return
	}
	store.mu.Lock()
	key := message.Chat.ID
	if message.From != nil {
		key = message.From.ID
	}
	user := store.users[key]
	if user == nil {
		user = &userSchedule{ChatID: message.Chat.ID}
		store.users[key] = user
	}
	user.TomorrowAt = args[0]
	store.mu.Unlock()
	sendText(bot, message.Chat.ID, "I will send tomorrow's lectures at "+args[0]+" Europe/Vilnius time.")
}

func handleICS(bot *tgbotapi.BotAPI, store *botStore, message *tgbotapi.Message) {
	store.mu.RLock()
	key := message.Chat.ID
	if message.From != nil {
		key = message.From.ID
	}
	user := store.users[key]
	if user == nil || user.Schedule.Week == nil {
		store.mu.RUnlock()
		sendText(bot, message.Chat.ID, "Send your timetable PDF first.")
		return
	}
	schedule := user.Schedule
	store.mu.RUnlock()
	tempFile, err := os.CreateTemp("", "happiness-*.ics")
	if err != nil {
		sendText(bot, message.Chat.ID, "Could not create the calendar file.")
		return
	}
	path := tempFile.Name()
	tempFile.Close()
	defer os.Remove(path)
	if err := writeICS(path, schedule); err != nil {
		sendText(bot, message.Chat.ID, "Could not create the calendar: "+err.Error())
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		sendText(bot, message.Chat.ID, "Could not read the calendar file.")
		return
	}
	document := tgbotapi.NewDocument(message.Chat.ID, tgbotapi.FileBytes{Name: "timetable.ics", Bytes: data})
	if _, err := bot.Send(document); err != nil {
		sendText(bot, message.Chat.ID, "Could not send the calendar: "+err.Error())
	}
}

func reminderLoop(bot *tgbotapi.BotAPI, store *botStore, location *time.Location) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for now := range ticker.C {
		localNow := now.In(location).Truncate(time.Minute)
		store.mu.Lock()
		for _, user := range store.users {
			if user.Schedule.Week == nil {
				continue
			}
			if user.ReminderBefore > 0 {
				target := localNow.Add(time.Duration(user.ReminderBefore) * time.Minute)
				for _, lecture := range lecturesOnDate(user.Schedule, target) {
					key := target.Format("2006-01-02 15:04") + "|" + lecture.SubjectName
					if key != user.LastReminder {
						user.LastReminder = key
						sendText(bot, user.ChatID, fmt.Sprintf("Reminder: %s starts at %s in %d minutes (%s).", lecture.SubjectName, target.Format("15:04"), user.ReminderBefore, lecture.Auditorium))
						break
					}
				}
			}
			if user.TomorrowAt != "" && localNow.Format("15:04") == user.TomorrowAt {
				key := localNow.Format("2006-01-02")
				if key != user.LastTomorrow {
					user.LastTomorrow = key
					lectures := lecturesOnDate(user.Schedule, localNow.AddDate(0, 0, 1))
					sendText(bot, user.ChatID, formatTomorrow(lectures, localNow.AddDate(0, 0, 1)))
				}
			}
		}
		store.mu.Unlock()
	}
}

func lecturesOnDate(schedule Schedule, date time.Time) []models.Lecture {
	if date.Before(schedule.StartDate) || date.After(schedule.EndDate) || date.Weekday() == time.Sunday || date.Weekday() == time.Saturday {
		return nil
	}
	day := schedule.Week[(int(date.Weekday())+6)%7]
	anchor := startOfWeek(time.Date(2026, 9, 16, 0, 0, 0, 0, date.Location()))
	weeksFromAnchor := int(date.Sub(anchor).Hours() / 24 / 7)
	weekNumber := ((weeksFromAnchor % 2) + 2) % 2
	var lectures []models.Lecture
	for _, lecture := range day {
		if lecture.Week == 0 || int(lecture.Week-1) == weekNumber {
			lectures = append(lectures, lecture)
		}
	}
	return lectures
}

func formatTomorrow(lectures []models.Lecture, date time.Time) string {
	if len(lectures) == 0 {
		return "No lectures tomorrow (" + date.Format("2006-01-02") + ")."
	}
	var lines []string
	for _, lecture := range lectures {
		lines = append(lines, fmt.Sprintf("%s-%s %s (%s)", lecture.StartTime.Format("15:04"), lecture.EndTime.Format("15:04"), lecture.SubjectName, lecture.Auditorium))
	}
	return "Tomorrow's lectures:\n" + strings.Join(lines, "\n")
}

func sendText(bot *tgbotapi.BotAPI, chatID int64, text string) {
	if _, err := bot.Send(tgbotapi.NewMessage(chatID, text)); err != nil {
		fmt.Printf("failed to send Telegram message to %d: %v\n", chatID, err)
	}
}
