package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Daxin319/Gator/internal/config"
	"github.com/Daxin319/Gator/internal/database"
	"github.com/google/uuid"

	_ "github.com/lib/pq"
)

func main() {
	// Read config file
	configFile := config.Read()

	// Ensure postgres is running
	database.EnsurePostgresRunning()

	// Check for db and create if none
	database.EnsureDatabaseExists()

	dsn := "host=localhost port=5432 user=postgres password=postgres dbname=gator sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		fmt.Println("ERROR: Cannot connect to PostgreSQL:", err)
	}
	defer db.Close()

	var connectedDB string
	err = db.QueryRow("SELECT current_database();").Scan(&connectedDB)
	if err != nil {
		log.Fatal("ERROR: Could not fetch connected database:", err)
	}

	fmt.Printf("GatorDB is ready!\n\n")
	fmt.Printf("------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------\n\n\n")

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
	commands.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	commands.register("feeds", handlerListFeeds)
	commands.register("follow", middlewareLoggedIn(handlerFollow))
	commands.register("following", middlewareLoggedIn(handlerFollowing))
	commands.register("unfollow", middlewareLoggedIn(handlerUnfollow))
	commands.register("browse", middlewareLoggedIn(handlerBrowse))

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
		return err
	}

	// Prepare the user creation parameters
	args := database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      cmd.arguments[0],
	}

	// Insert user into the database
	_, err := s.db.CreateUser(context.Background(), args)
	if err != nil {
		fmt.Println("ERROR: Failed to insert user into database:", err)
		return err
	}

	// Set the username in the config
	s.config.SetUser(cmd.arguments[0])

	fmt.Printf("Username set to %s\n", cmd.arguments[0])
	return nil
}

func handlerReset(s *state, cmd command) error {
	err := s.db.ResetUsers(context.Background())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("database reset!")
	return nil
}

func handlerList(s *state, cmd command) error {
	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		fmt.Println("error getting users from database", err)
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

func handlerListFeeds(s *state, cmd command) error {
	feeds, err := s.db.GetFeeds(context.Background())
	if err != nil {
		fmt.Println("error getting feeds from database")
		os.Exit(1)
	}

	for _, feed := range feeds {
		creator, err := s.db.GetCreator(context.Background(), feed.UserID)
		if err != nil {
			fmt.Println("error retreiving creator data")
		}
		fmt.Printf("- Feed: %s\n  URL: %s\n  Created by: %s\n\n", feed.Name, feed.Url, creator)
	}

	return nil
}

func handlerAgg(s *state, cmd command) error {
	if len(cmd.arguments) == 0 {
		fmt.Println("expecting one argument (time between requests: '1m', '8h', '30s' etc.)")
		os.Exit(1)
	}
	timeBetweenRequests, err := time.ParseDuration(cmd.arguments[0])
	if err != nil {
		fmt.Println("invalid time format")
		os.Exit(1)
	}
	if timeBetweenRequests < (time.Duration(5) * time.Second) {
		fmt.Println("too short of a time period. Don't DOS people.")
		os.Exit(1)
	}

	ticker := time.NewTicker(timeBetweenRequests)
	for ; ; <-ticker.C {
		scrapeFeeds(s)
	}
}

func handlerAddFeed(s *state, cmd command, user database.User) error {
	if len(cmd.arguments) < 2 {
		fmt.Println("not enough arguments provided, please provide a name and url")
		os.Exit(1)
	}

	user_id := user.ID

	args := database.CreateFeedParams{
		ID:            uuid.New(),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Name:          cmd.arguments[0],
		Url:           cmd.arguments[1],
		UserID:        user_id,
		LastFetchedAt: time.Now(),
	}

	feed, err := s.db.CreateFeed(context.Background(), args)
	if err != nil {
		fmt.Printf("error creating feed")
		os.Exit(1)
	}

	feed_id := feed.ID
	feed_name := feed.Name

	follow_args := database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    user_id,
		FeedID:    feed_id,
	}

	_, err = s.db.CreateFeedFollow(context.Background(), follow_args)
	if err != nil {
		fmt.Println("error creating feed_follow record")
		os.Exit(1)
	}

	fmt.Printf("%s has followed %s\n", user.Name, feed_name)

	fmt.Println(feed)
	return nil
}

func handlerFollow(s *state, cmd command, user database.User) error {
	if len(cmd.arguments) == 0 {
		fmt.Printf("expecting 1 argument (url)")
		os.Exit(1)
	}

	user_id := user.ID

	feed, err := s.db.URLLookup(context.Background(), cmd.arguments[0])
	if err != nil {
		fmt.Println("error getting feed id")
		os.Exit(1)
	}

	feed_id := feed.ID
	feed_name := feed.Name

	args := database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    user_id,
		FeedID:    feed_id,
	}

	_, err = s.db.CreateFeedFollow(context.Background(), args)
	if err != nil {
		fmt.Println("error creating feed_follow record")
		os.Exit(1)
	}

	fmt.Printf("%s has followed %s\n", user.Name, feed_name)

	return nil
}

func handlerFollowing(s *state, cmd command, user database.User) error {
	following, err := s.db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		fmt.Println("error getting followed feeds")
	}

	fmt.Printf("%s is following:\n\n", user.Name)

	for _, feed := range following {
		fmt.Printf("  -%s\n", feed.FeedName)
	}

	return nil
}

func handlerUnfollow(s *state, cmd command, user database.User) error {
	if len(cmd.arguments) == 0 {
		fmt.Println("expecting 1 argument (url)")
		os.Exit(1)
	}

	feed, _ := s.db.URLLookup(context.Background(), cmd.arguments[0])

	arg := database.DeleteFollowParams{
		Name: user.Name,
		Url:  cmd.arguments[0],
	}

	s.db.DeleteFollow(context.Background(), arg)
	fmt.Printf("You have unfollowed %s\n", feed.Name)

	return nil
}

func handlerBrowse(s *state, cmd command, user database.User) error {
	limit := 2
	if len(cmd.arguments) == 1 {
		userLimit, _ := strconv.Atoi(cmd.arguments[0])
		limit = userLimit - 1
	}

	posts, err := s.db.GetPostsForUser(context.Background(), user.ID)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if limit > len(posts) {
		limit = len(posts)
	}

	for i, post := range posts {
		if i > limit {
			break
		}
		fmt.Printf("\n- %s\n", post.Title)
		fmt.Printf(" - %s          %s\n\n", post.FeedTitle, post.PublishedAt)
		fmt.Printf(" %s\n", post.Description)
		fmt.Printf(" <Ctrl + LMB> to visit full article in browser -> %s\n\n\n", post.Url)
		fmt.Printf("------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------\n\n")
	}

	return nil
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		user, err := s.db.GetUser(context.Background(), s.config.CurrentUserName)
		if err != nil {
			fmt.Println("error getting user id")
			os.Exit(1)
		}
		handler(s, cmd, user)
		return nil
	}
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

func scrapeFeeds(s *state) {
	nextFeed, err := s.db.GetNextFeedToFetch(context.Background())
	if err != nil {
		fmt.Println("error getting next feed to fetch")
		os.Exit(1)
	}

	arg := database.MarkFeedFetchedParams{
		LastFetchedAt: time.Now(),
		ID:            nextFeed.ID,
	}

	err = s.db.MarkFeedFetched(context.Background(), arg)
	if err != nil {
		fmt.Println("error marking feed fetched")
		os.Exit(1)
	}

	feed, err := fetchFeed(context.Background(), nextFeed.Url)
	if err != nil {
		fmt.Println("error fetching feed")
	}

	for _, item := range feed.Channel.Item {
		formattedDate, err := parseTimeToRFC3339(item.PubDate)
		if err != nil {
			fmt.Println("ERROR unable to format date: ", err)
			os.Exit(1)
		}

		args := database.CreatePostParams{
			ID:          uuid.New(),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Title:       item.Title,
			Url:         item.Link,
			Description: item.Description,
			PublishedAt: formattedDate,
			FeedID:      nextFeed.ID,
		}

		_, err = s.db.CreatePost(context.Background(), args)
		if err != nil {
			if dup := strings.Contains(err.Error(), "unique"); dup {
				continue
			} else {
				fmt.Println(err)
			}

		}
	}
}

func parseTimeToRFC3339(input string) (string, error) {

	if input == "" {
		return "", nil // Handle empty input gracefully
	}

	// List of possible date formats
	formats := []string{
		time.RFC3339,                    // 2006-01-02T15:04:05Z07:00
		time.RFC1123,                    // Mon, 02 Jan 2006 15:04:05 MST
		time.RFC1123Z,                   // Mon, 02 Jan 2006 15:04:05 -0700
		"Mon, 02 Jan 2006 15:04:05 GMT", // Explicit RFC1123 with GMT
		"2006-01-02 15:04:05",           // MySQL DATETIME
		"2006-01-02",                    // YYYY-MM-DD
		"02 Jan 2006 15:04:05 MST",      // 02 Jan 2006 15:04:05 UTC
		"02 Jan 2006",                   // 02 Jan 2006
	}

	var parsedTime time.Time
	var err error
	for _, format := range formats {
		parsedTime, err = time.Parse(format, input)
		if err == nil {
			// Successfully parsed, return in RFC 3339 format
			return parsedTime.Format(time.RFC3339), nil
		}
	}

	// Debug output
	fmt.Printf("Failed to parse: %s\n", input)
	return "", fmt.Errorf("unable to parse time: %s", input)
}
