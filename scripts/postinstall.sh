#!/bin/bash
systemctl daemon-reload
systemctl enable webtop
systemctl restart webtop
