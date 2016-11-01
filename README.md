sentry-assign
=============

This is a tiny little bot to auto assign issues in Sentry to a default user
on a per-project basis. Currently, this project only supports interacting
with hosted Sentry at https://sentry.io.

installation
============

go get github.com/ttacon/sentry-assign


configuration
=============

You need the following to run sentry-assign:

 - A Sentry API token (from https://sentry.io/api).
 - A Sentry DSN (if you wish to test the auto assign functionality).
 - A configuration file with the following format:

```json
{
  "project0": "name@domain.com",
  "project1": "name2@domain.com"
}
```

running
=======
```sh
$ sentry-assign --help

  -api-token string
        Sentry API Token
  -assign-loc string
        JSON mapping of project to default assignee (default "./assignments.json")
  -bind-addr string
        Address to bind the web server to (default ":18091")
  -sentry-dsn string
        Sentry DSN
```

testing
=======
Assuming you provided a Sentry DSN, you can test with the following:

```sh
curl -XPOST http://$HOST:$PORT/fire -d `{"culprit":"What a cool culprit"}`
```