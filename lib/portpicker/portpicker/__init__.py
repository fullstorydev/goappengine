#!/usr/bin/python3
#
# Copyright 2007 Google Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
"""Pure python code for finding unused ports on a host.

This module provides a pick_unused_port() function.
It can also be called via the command line for use in shell scripts.
When called from the command line, it takes one optional argument, which,
if given, is sent to portserver instead of portpicker's PID.
To reserve a port for the lifetime of a bash script, use $BASHPID as this
argument.

There is a race condition between picking a port and your application code
binding to it.  The use of a port server to prevent that is recommended on
loaded test hosts running many tests at a time.

If your code can accept a bound socket as input rather than being handed a
port number consider using socket.bind(('localhost', 0)) to bind to an
available port without a race condition rather than using this library.

Typical usage:
  test_port = portpicker.pick_unused_port()
"""

import socket
import sys

# The legacy Bind, IsPortFree, etc. names are not exported.
__all__ = ('pick_unused_port')

_PROTOS = [(socket.SOCK_STREAM, socket.IPPROTO_TCP),
           (socket.SOCK_DGRAM, socket.IPPROTO_UDP)]


def bind(port, socket_type, socket_proto):
    """Try to bind to a socket of the specified type, protocol, and port.

    This is primarily a helper function for PickUnusedPort, used to see
    if a particular port number is available.

    For the port to be considered available, the kernel must support at least
    one of (IPv6, IPv4), and the port must be available on each supported
    family.

    Args:
      port: The port number to bind to, or 0 to have the OS pick a free port.
      socket_type: The type of the socket (ex: socket.SOCK_STREAM).
      socket_proto: The protocol of the socket (ex: socket.IPPROTO_TCP).

    Returns:
      The port number on success or None on failure.
    """
    got_socket = False
    for family in (socket.AF_INET6, socket.AF_INET):
        try:
            sock = socket.socket(family, socket_type, socket_proto)
            got_socket = True
        except socket.error:
            continue
        try:
            sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            sock.bind(('', port))
            if socket_type == socket.SOCK_STREAM:
                sock.listen(1)
            port = sock.getsockname()[1]
        except socket.error:
            return None
        finally:
            sock.close()
    return port if got_socket else None


def pick_unused_port(pid=None):
    """A pure python implementation of PickUnusedPort.

    Args:
      pid: PID to tell the portserver to associate the reservation with. If
        None,
        the current process's PID is used.

    Returns:
      A port number that is unused on both TCP and UDP.
    """
    return _pick_unused_port_without_server()


PickUnusedPort = pick_unused_port  # legacy API. pylint: disable=invalid-name


def _pick_unused_port_without_server():  # Protected. pylint: disable=invalid-name
    """Pick an available network port without the help of a port server.

    This code ensures that the port is available on both TCP and UDP.

    This function is an implementation detail of PickUnusedPort(), and
    should not be called by code outside of this module.

    Returns:
      A port number that is unused on both TCP and UDP.  None on error.
    """

    # Try OS-assigned ports next.
    # Ambrose discovered that on the 2.6 kernel, calling Bind() on UDP socket
    # returns the same port over and over. So always try TCP first.
    while True:
        # Ask the OS for an unused port.
        port = bind(0, _PROTOS[0][0], _PROTOS[0][1])
        # Check if this port is unused on the other protocol.
        if port:
            return port


def main(argv):
    """If passed an arg, treat it as a PID, otherwise portpicker uses getpid."""
    port = pick_unused_port(pid=int(argv[1]) if len(argv) > 1 else None)
    if not port:
        sys.exit(1)
    print(port)


if __name__ == '__main__':
    main(sys.argv)
