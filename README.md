# Gator
RSS Blog Aggregator CLI tool written in Go

To run this program you will need the latest version of the Go toolchain and Postgres installed on your machine to install the binary.

To install the latest version of the Go toolchain, visit https://go.dev/doc/install and select the version appropriate for your OS.

To install Postgres, visit https://www.postgresql.org/download/ and select the version appropriate for your OS.

After you have installed Go and Postgres, open your terminal/shell and run `go install github.com/Daxin319/Gator@latest`. Once the program has installed, run it with `gator`

If this is the first time you're launching gator, you'll need to register a user. Run `gator register <username>` to register a new user. If you have more than 1 user, you can use `gator login <username>` to change users.

To reset the database and clear all data, run `gator reset`

To see a list of users, run `gator users`

To see a list of all RSS feeds currently stored in the database, run `gator feeds`

To see a list of RSS feeds currently followed by the current user, run `gator following`

To add an RSS feed to the database, run `gator addfeed <feed_name> <feed_url>`. the feed_name cannot contain spaces. This will also cause the current user to follow the RSS feed.

If an RSS feed already exists, you can follow it for the current user with `gator follow <feed_url>`

You can unfollow a feed with `gator unfollow <feed_url>`

To begin content aggregation, run `gator agg <time_between_requests>` where time_between_requests is formatted like "30s", "1h", "3.5h", "20m" etc.

To browse aggregated stories, run `gator browse <optional_limit>`. If no limit provided it will default to the 3 most recent items.



