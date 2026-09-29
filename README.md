# gobots

`gobots` is a software package that introduces the concept of `botfiles`
which describe commands, tasks and workflows executed by *robots*.

Admittedly, the term `robots` may be somewhat "broad". This project is a direct
result of countless nights spent trying to *talk* with specific Arduino boards.

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