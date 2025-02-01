package main

import (
	"database/sql"
	"gopkg.in/telebot.v4"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type Reminder struct {
	ID      int
	UserID  int64
	Message string
	Time    string
	Status  string
}

var db *sql.DB

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	var err error
	db, err = sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}

	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			log.Println("Error closing db")
		}
	}(db)

	bot, err := telebot.NewBot(telebot.Settings{
		Token:  os.Getenv("BOT_TOKEN"),
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Fatal(err)
	}

	menu := &telebot.ReplyMarkup{}
	btnStart := menu.Text("/start")
	btnRemind := menu.Text("/remind")
	menu.Reply(menu.Row(btnStart, btnRemind))

	bot.Handle("/start", func(c telebot.Context) error {
		return c.Send("Hello", menu)
	})

	bot.Handle("/remind", func(c telebot.Context) error {
		err := c.Send("Enter your reminder message:")
		if err != nil {
			return err
		}
		bot.Handle(telebot.OnText, func(c telebot.Context) error {
			message := c.Text()
			userID := c.Sender().ID

			err := c.Send("Now enter the time (Irden, Gunortan, Agsham + hour:minute or hour):")
			if err != nil {
				return err
			}
			bot.Handle(telebot.OnText, func(c telebot.Context) error {
				timeInput := c.Text()
				parsedTime := parseTime(timeInput)
				if parsedTime == "" {
					return c.Send("Invalid time format. Try again!")
				}

				_, err := db.Exec("INSERT INTO reminders (user_id, message, time, status) VALUES ($1, $2, $3, 'pending')", userID, message, parsedTime)
				if err != nil {
					log.Println("DB Error:", err)
					return c.Send("Failed to save reminder.")
				}

				return c.Send("Reminder saved! I'll remind you at " + parsedTime)
			})
			return nil
		})
		return nil
	})

	go reminderScheduler(bot)

	bot.Start()
}

func parseTime(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	timeFormats := map[string]string{
		"irden":    "07:00",
		"gunortan": "12:00",
		"agsham":   "18:00",
	}

	for key, val := range timeFormats {
		if strings.Contains(input, key) {
			timeRegex := regexp.MustCompile(`\d{1,2}:?\d{0,2}`)
			matches := timeRegex.FindString(input)
			if matches != "" {
				return matches
			}
			return val
		}
	}
	return ""
}

func reminderScheduler(bot *telebot.Bot) {
	for {
		rows, err := db.Query("SELECT id, user_id, message, time FROM reminders WHERE status='pending'")
		if err != nil {
			log.Println("DB Query Error:", err)
			continue
		}

		var reminders []Reminder
		for rows.Next() {
			var r Reminder
			if err := rows.Scan(&r.ID, &r.UserID, &r.Message, &r.Time); err == nil {
				reminders = append(reminders, r)
			}
		}
		err = rows.Close()
		if err != nil {
			return
		}

		for _, r := range reminders {
			parsedTime, _ := time.Parse("15:04", r.Time)
			if time.Now().Format("15:04") == parsedTime.Format("15:04") {
				_, err := bot.Send(&telebot.User{ID: r.UserID}, "Reminder: "+r.Message)
				if err != nil {
					return
				}
				_, _ = db.Exec("UPDATE reminders SET status='sent' WHERE id=$1", r.ID)
			}
		}
		time.Sleep(10 * time.Second)
	}
}
