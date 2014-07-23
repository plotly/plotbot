# ahipbot -- A simple bot written in Go

[![Build Status](https://drone.io/github.com/abourget/ahipbot/status.png)](https://drone.io/github.com/abourget/ahipbot/latest)


## Configuration

* Install your Go environment, under Ubuntu, use this method:

    http://blog.labix.org/2013/06/15/in-flight-deb-packages-of-go

* Copy the `dot.hipbot` file to `$HOME/.hipbot` and tweak until you're pleased.

* Build and run with: `./hipbot`

* Enjoy!


## Writing your own plugin

Take inspiration by looking at `funny.go`.  Write your own, and don't forget to
add your plugin to `Hipbot.registerPlugins()` in `hipbot.go`.
