# The Plotly Bot -- A simple bot written in Go

[![Build Status](https://drone.io/github.com/plotly/plotbot/status.png)](https://drone.io/github.com/plotly/plotbot/latest)


## Local installation without docker container

You can either install locally the go environment, or simply use go docker image to create a go docker container. For the former approach, please read the following guide; jump to next [section](#using-go-docker-container-instead) for the go docker image approach.

### Setup your own Go environment

* Install your Go environment, under Ubuntu, use this method:

    http://blog.labix.org/2013/06/15/in-flight-deb-packages-of-go

* Set your `GOPATH`:

    On Ubuntu see [here](http://stackoverflow.com/questions/21001387/how-do-i-set-the-gopath-environment-variable-on-ubuntu-what-file-must-i-edit/21012349#21012349)


* Install Ubuntu dependencies needed by various steps in this document:

    ```sudo apt-get install mercurial zip```

* Pull the bot and its dependencies:

    ```go get github.com/plotly/plotbot/plotbot```

* Install rice:

    ```go get github.com/GeertJohan/go.rice/rice```

* Run "npm install":

   ```
   cd $GOPATH/src/github.com/plotly/plotbot/web
   npm install
   ```

* Run "npm run build":

   ```
   cd $GOPATH/src/github.com/plotly/plotbot/web
   npm run build
   ```

### Local build and install

* Copy the `plotbot.sample.conf` file to `$HOME/.plotbot` and tweak at will.

* Build with:

   ```
   cd $GOPATH/src/github.com/plotly/plotbot/plotbot
   go build && ./plotbot
   ```
   
* Note: It is also possible to build plotbot using the stable dependencies found
        within the Godeps directory. This can be done as follows: 
        
        Install godep: 
        
           go get github.com/tools/godep
           
        Now build using the godep tool as follows:
        
           cd $GOPATH/src/github.com/plotly/plotbot/plotbot
           godep go build && ./plotbot
              
                   
* Inject static stuff (for the web app) in the binary with:

   ```
   cd $GOPATH/src/github.com/plotly/plotbot/web
   rice append --exec=../plotbot/plotbot
   ```

* Enjoy! You can deploy the binary and it has all the assets in itself now.


## Using Go docker container instead

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
