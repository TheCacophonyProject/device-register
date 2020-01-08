# device register

`device-register` registers a device on the cacophony server so a device can upload thermal or audio recordings to the server

Project | device-register
---|--- |
Platform | Thermal camera (Raspbian) |
Requires | Running [`cacophony-api`](https://github.com/TheCacophonyProject/cacophony-api) server to connect to |
Build Status | [![Build Status](https://api.travis-ci.com/TheCacophonyProject/device-register.svg?branch=master)](https://travis-ci.com/TheCacophonyProject/device-register) |
Licence | GNU General Public License v3.0 |

## Instructions

Download and install the latest release from [Github](https://github.com/TheCacophonyProject/device-register/releases)

In order to create register the device, you need to specify the path the API server, group name the device will belong to, and name for the device.  Up-to-date instructions for how to specify these values can be found by running.
```
> ./device-register --help
```

## Development Instructions

Follow our [go instructions](https://docs.cacophony.org.nz/home/developing-in-go) to download and build this project.

Make sure the [`cacophony-api`](https://github.com/TheCacophonyProject/cacophony-api) server the device will attach to is running, then start this program.   Up-to-date instructions for how to specify these values can be found by running.
```
> ./device-register --help
```

Releases are created using travis and git and saved [on Github](https://github.com/TheCacophonyProject/device-register/releases).   Follow our [release instructions](https://docs.cacophony.org.nz/home/creating-releases) to create a new release.

