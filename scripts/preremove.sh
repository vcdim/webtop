#!/bin/bash
systemctl stop webtop 2>/dev/null || true
systemctl disable webtop 2>/dev/null || true
