#!/bin/sh
# Docksmith sample application
# Demonstrates ENV override and filesystem isolation

echo "================================"
echo "  $GREETING from $APP_NAME!"
echo "================================"
echo ""
echo "Environment:"
echo "  APP_NAME = $APP_NAME"
echo "  GREETING = $GREETING"
echo "  PWD      = $(pwd)"
echo ""
echo "Build log:"
cat /app/build.log
echo ""
echo "Message:"
cat /app/message.txt
echo ""
echo "Container running successfully!"
