# blackbox
* Slack: #syslog on <https://slack.cloudfoundry.org>
* Tracker: [CF Platform Logging Improvements][tracker]
* CI: [Syslog CI][CI]

## About
Blackbox will tail all files in sub-directories of a specified `source_dir`, and forward any new lines to a syslog server.

This is currently used in [syslog-release][syslog] and [windows-syslog-release][windows-syslog]. 
## Usage

```
blackbox -config config.yml
```

The configuration file schema is as follows:

``` yaml
hostname: this-host

syslog:
  destination:
    transport: udp
    address: logs.example.com:1234

  source_dir: /path/to/log-dir
```

Consider the case where `log-dir` has the following structure:

```
/path/to/log-dir
|-- app1
|   |-- stdout.log
|   `-- stderr.log
`-- app2
    |-- foo.log
    `-- bar.log
```

Any new lines written to `app1/stdout.log` and `app1/stderr.log` get sent to syslog tagged as `app1`, while new lines written to `app2/foo.log` and `app2/bar.log` get sent to syslog tagged as `app2`.

Currently the priority and facility are hardcoded to `INFO` and `user`.

## Installation

```
go get -u github.com/cloudfoundry/blackbox/cmd/blackbox
```

[CI]: https://syslog.ci.cf-app.com
[tracker]: https://www.pivotaltracker.com/n/projects/2126318
[windows-syslog]: https://github.com/cloudfoundry/windows-syslog-release
[syslog]: https://github.com/cloudfoundry/syslog-release
