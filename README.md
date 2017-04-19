# The Plotly Bot -- A simple bot written in Go

[![Build Status](https://drone.io/github.com/plotly/plotbot/status.png)](https://drone.io/github.com/plotly/plotbot/latest)

## Install Latest Golang Environment
Plently of existing documentation on this.

## Installation and Development
### Install with
```
go get github.com/plotly/plotbot
```

### Build with
```
cd $GOPATH/src/github.com/plotly/plotbot/plotbot
go build && ./plotbot
```

### Configure with
configuration file found in `plotly/deployment`. Talk to Ben or Jody about configuring plotbot.


### Dependency management
Plotbot uses vendored assets. When updating an asset make sure to check it into the vendor folder. Until `dep` is released as an official Go package manager we are using `govendor`.


## Using Go docker container instead [NOTE MAY BE OUT OF DATE]

Rename `docker-compose.yml.example` file to `docker-compose.yml`, and tweak at will. For example, change the go version. The GOPATH is set under container, so no need do it, the current working `plotbot` folder will be mounted inside container as `/go/src/github.com/plotly/plotbot`, so no need to set project path either. As docker container synchronizes such folder with your filesystem, so you can just edit files on your local filesystem.

* Use `docker-compose` to run container, under project directory

    ```bash
    $ docker-compose up -d  # run container plotbot
    $ docker-compose stop   # stop container plotbot
    $ docker-compose start  # start stopped container plotbot
    $ docker-compose rm -f  # delete container plotbot
    ```
  **PS**: `docker-compose rm -f` will delete all efforts done inside container, while `stop/start` routine will keep all the changes.

* To run inside container plotbot

    ```bash
    $ docker exec -ti plotbot /bin/bash
    ```

* To install godep, inside container plotbot do

    ```bash
    # go get github.com/tools/godep
    ```

* Compile plotbot source inside docker container,

    ```bash
    # cd /go/src/github.com/plotly/plotbot/plotbot
    # godep go install
    ```

    above commands will create a binary `plotbot` file under `/go/bin` inside docker container, as the `GOPATH` is already set, you can run `plotbot` anywhere inside docker container.


## Writing your own plugin

Take inspiration by looking at the different plugins, like `Funny`,
`Healthy`, `Storm`, `Deployer`, etc..  Don't forget to update your
bot's plugins list, like `plotbot/main.go`
