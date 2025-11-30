This project, Harbor, is a GUI application for Docker. It uses the Docker API to displays the containers (grouped by project), images, networks, and volumes.

The user can interact with the Docker API:
- Delete images, containers, networks, and volumes
- Start and stop containers
- Create and delete volumes
- Create and delete networks
- Read container logs
- Open a shell in a container

Use the Swagger documentation `v1.51.yaml` to check the API. Use the versioned API e.g. `/v1.51/containers/json` to ensure compatibility.

This should be similar to OrbStack for Mac. Harbor should use an existing Docker daemon. It does not install its own Docker daemon.

If you need to vendor libraries, use the `vendor` directory. Don't modify code in the `vendor` directory.

Keep the functions small and focused. Use CLEAN code principles. Create new files and directories as needed to keep everything organized.

Use the UNIX socket on Linux and macOS. Use the named pipe on Windows. Avoid spreading OS specific code throughout the codebase.
