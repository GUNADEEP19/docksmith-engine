
#!/bin/sh
set -eu

echo "Hello from Docksmith sample app!"
echo "Working directory: $(pwd)"
echo "MESSAGE=$MESSAGE"

if [ -f /app/build.txt ]; then
  echo "Build artifact: $(cat /app/build.txt)"
else
  echo "Build artifact: (missing)"
fi

