---
name: devbox-status
description: Check GPU and port status on a remote host — who is running what, GPU utilization, memory, and listening ports. Use when the user asks about GPU status, port status, devbox status, or who is using the machine.
argument-hint: "[hostname]"
allowed-tools: Bash(ssh *)
---

# Devbox Status Check

Query system status on the remote host `$ARGUMENTS` (default: `ailab1` if no argument given).

## GPU Status

Run via SSH:

```
ssh $ARGUMENTS 'nvidia-smi --query-gpu=index,name,utilization.gpu,memory.used,memory.total --format=csv,noheader,nounits && echo "---PROCESSES---" && nvidia-smi --query-compute-apps=gpu_uuid,pid,process_name,used_gpu_memory --format=csv,noheader,nounits && echo "---USERS---" && nvidia-smi --query-compute-apps=pid --format=csv,noheader | xargs -I{} ps -o user= -p {} 2>/dev/null | sort | uniq -c | sort -rn'
```

Present:
1. **GPU Overview**: index, name, utilization %, memory used/total
2. **Who is using what**: user, GPU index, process name, PID, memory. Group by user.
3. **Summary**: GPUs in use vs idle, top users by memory

## Port Status

Run via SSH:

```
ssh $ARGUMENTS 'sudo ss -tulnp | head -50'
```

Present:
1. **Listening Ports**: port, protocol, process name, PID, user, address
2. **Notable services**: highlight well-known ports (22, 80, 443, 8080, 9999, etc.)

## Output Format

Format as readable tables. Mark idle GPUs. Keep it concise.
