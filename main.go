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
	"strconv"
	"strings"
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
	if timeBetweenRequests < (time.Duration(30) * time.Second) {
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
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      cmd.arguments[0],
		Url:       cmd.arguments[1],
		UserID:    user_id,
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

	for i, post := range posts {
		if i > limit {
			break
		}
		fmt.Printf("- %s\n", post.Title)
		fmt.Printf("   %s\n", post.Description)
		fmt.Printf("   %s\n\n", post.Url)
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

	nullTime := sql.NullTime{
		Time:  time.Now(),
		Valid: true,
	}

	arg := database.MarkFeedFetchedParams{
		LastFetchedAt: nullTime,
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
		args := database.CreatePostParams{
			ID:          uuid.New(),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Title:       item.Title,
			Url:         item.Link,
			Description: item.Description,
			PublishedAt: item.PubDate,
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
