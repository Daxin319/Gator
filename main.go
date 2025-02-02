package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"time"

	config "github.com/Daxin319/Gator/internal/config"
	"github.com/Daxin319/Gator/internal/database"
	"github.com/google/uuid"

	_ "github.com/lib/pq"
)

func main() {
	configFile := config.Read()

	db, err := sql.Open("postgres", configFile.DbURL)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	dbQueries := database.New(db)

	currentState := state{
		dbQueries,
		&configFile,
	}

	commands := commands{
		make(map[string]func(*state, command) error),
	}

	commands.register("login", handlerLogins)
	commands.register("register", handlerRegister)
	commands.register("reset", handlerReset)
	commands.register("users", handlerList)
	commands.register("agg", handlerAgg)

	args := os.Args

	if len(args) < 2 {
		err := fmt.Errorf("not enough arguments")
		fmt.Println(err)
		os.Exit(1)
	}

	command := command{
		args[1],
		args[2:],
	}

	commands.run(&currentState, command)
}

type state struct {
	db     *database.Queries
	config *config.Config
}

type command struct {
	name      string
	arguments []string
}

type commands struct {
	validCommands map[string]func(*state, command) error
}

func (c *commands) register(name string, f func(*state, command) error) {
	c.validCommands[name] = f
}

func (c *commands) run(s *state, cmd command) error {
	err := c.validCommands[cmd.name](s, cmd)
	if err != nil {
		return fmt.Errorf("unknown command")
	}

	return nil
}

type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

func handlerLogins(s *state, cmd command) error {
	if len(cmd.arguments) == 0 {
		err := fmt.Errorf("only one username expected")
		fmt.Println(err)
		os.Exit(1)
	}

	if _, err := s.db.GetUser(context.Background(), cmd.arguments[0]); err != nil {
		fmt.Println("user does not exist")
		os.Exit(1)
	}
	s.config.SetUser(cmd.arguments[0])
	fmt.Printf("Username set to %s\n", cmd.arguments[0])
	return nil
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.arguments) == 0 {
		err := fmt.Errorf("expecting an argument")
		fmt.Println(err)
		os.Exit(1)
	}

	args := database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      cmd.arguments[0],
	}

	if _, err := s.db.GetUser(context.Background(), args.Name); err == nil {
		fmt.Println("user already exists")
		os.Exit(1)
	}

	s.db.CreateUser(context.Background(), args)
	s.config.SetUser(cmd.arguments[0])
	fmt.Printf("Username set to %s\n", cmd.arguments[0])
	fmt.Printf("%s registered to database\n", cmd.arguments[0])
	return nil
}

func handlerReset(s *state, cmd command) error {
	err := s.db.ResetUsers(context.Background())
	if err != nil {
		fmt.Println("error resetting database")
		os.Exit(1)
	}
	fmt.Println("database reset!")
	return nil
}

func handlerList(s *state, cmd command) error {
	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		fmt.Println("error getting users from database")
		os.Exit(1)
	}
	for _, user := range users {
		if user == s.config.CurrentUserName {
			fmt.Printf("* %s (current)\n", user)
		} else {
			fmt.Printf("* %s\n", user)
		}
	}
	return nil
}

func handlerAgg(s *state, cmd command) error {
	fetched, err := fetchFeed(context.Background(), "https://www.wagslane.dev/index.xml")
	if err != nil {
		fmt.Println("error fetching RSS feed")
		os.Exit(1)
	}
	fmt.Println(*fetched)
	return nil
}

func fetchFeed(c context.Context, feedURL string) (*RSSFeed, error) {
	client := http.Client{}

	req, err := http.NewRequestWithContext(c, "GET", feedURL, nil)
	if err != nil {
		fmt.Println("error making request")
		os.Exit(1)
	}

	req.Header.Set("User-Agent", "gator")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("error performing request")
		os.Exit(1)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("error reading xml body")
		os.Exit(1)
	}

	feedStruct := RSSFeed{}

	err = xml.Unmarshal(body, &feedStruct)
	if err != nil {
		fmt.Println("error unmarshalling xml")
	}

	feedStruct.Channel.Title = html.UnescapeString(feedStruct.Channel.Title)
	feedStruct.Channel.Description = html.UnescapeString(feedStruct.Channel.Description)

	for _, item := range feedStruct.Channel.Item {
		item.Title = html.UnescapeString(item.Title)
		item.Description = html.UnescapeString(item.Description)
	}

	return &feedStruct, nil
}
