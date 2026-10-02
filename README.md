# evias/gobots

`gobots` is a software package that introduces the concept of `botfiles`
which describe commands, tasks and workflows executed by *robots*,
or any type of edge devices.

[![License](https://img.shields.io/badge/License-3--Clause%20BSD-blue.svg)][./LICENSE]
![Go version](https://img.shields.io/github/go-mod/go-version/evias/gobots)
[![v0.x-dev](https://img.shields.io/badge/evias/gobots-v0.x--dev-blue?logo=github)](https://evi.as)

Admittedly, the term `robots` may be somewhat "broad". This project is a direct
result of countless nights spent trying to *talk* with specific Arduino boards.

In short: **Sick of reverse-engineering communication protocols for your devices?**
Now is the time to switch to `gobots`, probability has it that at least someone
on the Internet has already described/mapped your edge device into a `botfile`.

## What Is Gobots?

Package gobots defines structures and interfaces to connect and communicate
with robot devices and/or any connected edge devices.

The `gobots` software introduces **botfile**s and the **Wire protocol** to
facilitate inter-devices communication over network or serial connections.

- The `robot.Robot` structure provides a connection wrapper and communication
channel for connected edge devices. This implementation is agnostic to the
actual edge devices' hardware and installed firmware.
- A `cmd` package is provided which serves as a first draft interface for
connection and communication with supported edge devices through YAML driver
files and the Wire messaging format.

## Usage

A terminal user interface is provided with an interactive console to connect
directly with edge devices and execute commands:

```bash
# Open an interactive console to a device using a driver file
$ gobots console drivers/ELEGOO/smartcar-v4.yaml

# Open an interactive console to a device using a custom host and port
$ gobots console 192.168.4.1:100 -d drivers/ELEGOO/smartcar-v4.yaml

# Execute commands with parameters from the console directly
bot> exec move --speed 10 --direction forward
bot> exec stop
```

Connect and execute commands directly with `gobots exec`:

```bash
$ gobots exec move --speed 10 --direction forward -d drivers/ELEGOO/smartcar-v4.yaml
```

Execute runnable flows with `gobots run`:

```bash
$ gobots run flows/ELEGOO/smartcar-v4/parking.yaml
```

## License

Copyright 2025-2026 Grégory Saive <greg@evi.as> for re:Software S.L. (resoftware.es).

Licensed under the [3-Clause BSD License](./LICENSE).
