# Gator
RSS Blog Aggregator CLI tool written in Go

Until I can get the windows branch working, if you want to run this program on a windows pc you will need to use WSL2. This is very simple as it's a package directly from microsoft.

Open your command line with `<Win + R>` then type `cmd` and press `<Enter>`. This will open the command line, now type or copy and paste the following command into your command line:

```
wsl --install
```
then reboot your pc when it finishes installing. After it finishes installing a window called `Ubuntu` should automatically open. If not you may need to open it manually by pressing `<Win>` and typing `Ubuntu`. You should now have access to a linux terminal! You can open this terminal by pressing `<Win>` and typing `terminal` or by opening the command line or powershell and using the GUI to open a new terminal of any type.


To run this program you will need the latest version of the Go toolchain and Postgres installed on your machine to install the binary.
Most users will find it easiest to use curl to install it.

To install the latest version of the Go toolchain, run the following command or visit https://go.dev/doc/install and select the version appropriate for your OS.
```
curl -sS https://webi.sh/golang | sh
```
if you don't already have curl installed, you can get it with
```
sudo apt update
sudo apt install curl
```

Gator requires at least version 16.6 of PostgreSQL to run.

To install postgres, run the following commands
```
sudo apt update
sudo apt install postgresql postgresql-contrib
```

Finally you'll need to enter a password, run the following command. The default server configuration is "postgres://postgres:postgres@localhost:5432/gator?sslmode=disable". where the 3rd "postgres" is the password for the database. When you set your password, if you choose `postgres` then the program will work right out of the box. Otherwise, run the program once to generate a .gatorconfig.json in your home directory. Use a text editor of your choice to edit this file and change ***ONLY*** the url password section to whatever password you set here. DO NOT FORGET THIS PASSWORD. Although possible to get back into your postgres database, it is significantly more difficult than remembering a password. 
```
sudo passwd postgres
```

After setting the password, start the postgres server in the background using 
```
sudo service postgresql start
```

The final step is you need to set a user that Gator can connect to. Enter your database with 
```
sudo -u postgres psql
```

once you're in your database, run the following command:
```
ALTER USER postgres PASSWORD 'postgres';
```
and finally type `exit` to return to the terminal.

Modify your config file if you set a different password, and you're ready to use Gator!


After you have installed Go and Postgres, open your terminal/shell and run
```
go install github.com/Daxin319/Gator@latest
```

Once the program has installed, run it with `Gator [command] [arguments]`


If this is the first time you're launching Gator, you'll need to register a user. Run `Gator register [username]` to register a new user. If you have more than 1 user, you can use `Gator login <username>` to change users.

To reset the database and clear all data, run `Gator reset`

To see a list of users, run `Gator users`

To see a list of all RSS feeds currently stored in the database, run `Gator feeds`

To see a list of RSS feeds currently followed by the current user, run `Gator following`

To add an RSS feed to the database, run `Gator addfeed [feed_name] [feed_url]`. The feed\_name cannot contain spaces. This will also cause the current user to follow the RSS feed.

If an RSS feed already exists, you can follow it for the current user with `Gator follow [feed_url]`

You can unfollow a feed with `Gator unfollow [feed_url]`

To begin content aggregation, run `Gator agg [time_between_requests]` where time\_between\_requests is formatted like "30s", "1h", "3.5h", "20m" etc.

To browse aggregated stories, run `Gator browse [optional_limit]`. If no limit provided it will default to the 3 most recent items.



