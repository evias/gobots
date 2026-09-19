/*
Package gobots defines structures and interfaces to connect and communicate
with robot devices and/or any connected device.

# Runtime

The `gobots` software introduces **botfile**s and the **Wire protocol** to
facilitate inter-devices communication over network or serial connections.

- The [robot.Robot] structure provides a connection wrapper and communication
channel for connected edge devices. This implementation is agnostic to the
actual edge devices' hardware and installed firmware.
- A `cmd` package is provided which serves as a first draft interface for
connection and communication with supported edge devices through YAML driver
files and the Wire messaging format.

# Drivers: botfiles

The `botfile` package provides an implementation for [Driver] files that are
YAML-based configuration files to describe communication with specific devices.

Botfiles define a series of instructions necessary to connect to specific edge
devices and to communicate messages over the established connection.

# The Wire Protocol

This protocol defines the formatting rules or guidelines to construct
messages, which may hold additional arguments/parameters/fields to parse.

The aim of this protocol is to make communication with devices more
configurable once connection is established. Before the Wire protocol sending
a "move forward for 2 seconds" command to a device would need exact bytes
written in a programming language that may be obscure for some tinklers and
prototypers. With the Wire protocol, you write these exact bytes in a driver
file, and may use commands by their name afterwards, i.e. you won't need to
programmatically build up exact bytes anymore later on.

Many robot devices for the educational domain already accept/understand some
level of JSON formats, e.g. ELEGOO robots. Yet, this protocol is intended
for use with binary message formats as well, depending on the needs of the
underlying devices with which you want to communicate.

A [Message] structure is provided which is intentionally simplistic in that
it basically defines a wrapper around [tmpl.Template] which may be returned
in its raw bytes-representation, with arguments/parameters parsed/filled.

e.g. Format = '{"N":102,"D1":{{.Speed}},"D2":{{.Direction}}}'

# Flows

A [botfile.Flow] implementation is provided which defines executable flows
that are made of a series of commands and their parameters.

A flow represents the reproducible execution of one or many commands,
e.g. a "parking" flow involves the execution of many different actions where
it will first determine a parking slot, then drive and park into it; another
example is a "greeting" flow which involves only the action of moving the
robot's arm in a greeting movement back-and-forth.

YAML-based flow definition is still under active development.
*/
package gobots
