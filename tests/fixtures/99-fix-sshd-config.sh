#!/bin/bash
# Increase MaxStartups and MaxSessions for testing
echo "MaxStartups 100:30:200" >> /etc/ssh/sshd_config
echo "MaxSessions 100" >> /etc/ssh/sshd_config
# Reload sshd to pick up the changes
pkill -HUP sshd 2>/dev/null || true
