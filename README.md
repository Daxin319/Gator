# Gator
RSS Blog Aggregator CLI tool written in Go



To run this program you will need the latest version of the Go toolchain and Postgres installed on your machine to install the binary.
Most users will find it easiest to use curl to install it.
For linux, the snap version of curl is worthless, you need to get the apt version with 
```
sudo apt update
sudo apt install curl
```
then run
```
curl -sS https://webi.sh/golang | sh
```

To install the latest version of the Go toolchain, or visit https://go.dev/doc/install and select the version appropriate for your OS.



To install PostgreSQL, visit https://www.postgresql.org/download/ and select the version appropriate for your OS.
Gator requires at least version 16.6 of PostgreSQL to run.

On Windows, you may need to add the binary folder to your path, open a powershell as administrator and run the following command to add it to your path:

`[System.Environment]::SetEnvironmentVariable("Path", $env:Path + ";C:\Program Files\PostgreSQL\17\bin\", [System.EnvironmentVariableTarget]::Machine)` Version 17 is the current release at time of writing, if you have a newer version of postgres, change the 17 to your version number.



After you have installed Go and Postgres, open your terminal/shell and run `go install github.com/Daxin319/Gator@latest`. Once the program has installed, run it with `Gator`

If this is the first time you're launching Gator, you'll need to register a user. Run `Gator register <username>` to register a new user. If you have more than 1 user, you can use `Gator login <username>` to change users.

To reset the database and clear all data, run `Gator reset`

To see a list of users, run `Gator users`

To see a list of all RSS feeds currently stored in the database, run `Gator feeds`

To see a list of RSS feeds currently followed by the current user, run `Gator following`

To add an RSS feed to the database, run `Gator addfeed <feed_name> <feed_url>`. the feed_name cannot contain spaces. This will also cause the current user to follow the RSS feed.

If an RSS feed already exists, you can follow it for the current user with `Gator follow <feed_url>`

You can unfollow a feed with `Gator unfollow <feed_url>`

To begin content aggregation, run `Gator agg <time_between_requests>` where time_between_requests is formatted like "30s", "1h", "3.5h", "20m" etc.

To browse aggregated stories, run `Gator browse <optional_limit>`. If no limit provided it will default to the 3 most recent items.



