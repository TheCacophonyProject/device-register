#!/bin/bash
systemctl daemon-reload
systemctl enable device-register.service
systemctl restart device-register.service