# JTTY-Go NET rigctl radio control

JTTY-Go connects directly to an already running `rigctld` service over TCP. The client sends the rigctld text protocol commands for frequency (`f`/`F`), mode (`m`/`M`), PTT (`T`), split operation, and optional meter reads.

The application does not launch `rigctld.exe`, load Hamlib DLLs, enumerate Hamlib models, or configure the remote server's CAT serial parameters. Configure the remote `rigctld` process separately, then enter its host and TCP port in the Radio settings page. The normal rigctld port is 4532.

PTT modes are VOX, Rigctl, RTS, and DTR. VOX is triggered by outgoing audio. Rigctl sends PTT commands over the configured TCP connection. RTS and DTR use a local serial port on the JTTY-Go computer and can be tested without connecting to a rigctld server.

Legacy settings using `CAT` are migrated to `Rigctl`. Former local Hamlib settings are migrated to the NET endpoint setting when the old radio configuration was enabled; check the host and port after upgrading.
