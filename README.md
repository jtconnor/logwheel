# logwheel
Reads logs from stdin and writes them to rotating files.  Useful for log
rotation for 12-factor apps that write their logs to stdout
(http://12factor.net/logs).

Usage:
```
$ ./app | ./logwheel --log /var/log/app/log
```
