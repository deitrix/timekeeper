# Timekeeper

## Installation

```shell
go install github.com/deitrix/timekeeper/cmd/tk@latest
```

## How to use

### Start tracking a new task

```shell
$ tk start "Task name"

Started: Task name (ref=0)
```

### Stop tracking the current task

```shell
$ tk stop

Stopped: Task name (ref=0)
    
Duration   20 seconds
This week  20 seconds
Total      20 seconds
```

### Restart the last task (reference 0)

```shell
$ tk start

Started: Another task (ref=0)
```

### View the current task (affected by start and stop)

```shell
$ tk

In progress: Another task (ref=0)

Duration   2 seconds
This week  2 seconds
Total      2 seconds
```

### Archive (or unarchive) a task. Archived tasks are not included in the task list by default.

```shell
$ tk archive 11

Archived: Task name (id=11)
```

### Or archive the current task

```shell
$ tk a

Archived: Another task (ref=0)
```

### List tasks.

Each task has a reference number that can be used to perform actions on that task. For the first 10
tasks, the reference number is the same as the index in the list, to make it easier to reference
recent tasks. After 10 tasks, the reference number is a unique identifier which is generated on task
creation.

```shell
$ tk ls

Ref        Name       Last Start     Last Duration  This Week   Total
0 (id=11)  Task name  2 minutes ago  6 seconds      7 minutes   7 minutes
1 (id=16)  Test       2 minutes ago  7 seconds      23 seconds  23 seconds
2 (id=14)  Test       2 minutes ago  2 seconds      37 seconds  37 seconds
```

### List archived tasks

```shell
$ tk ls -a

Ref        Name          Last Start      Last Duration  This Week   Total       Archived
0 (id=11)  Task name     2 minutes ago   6 seconds      7 minutes   7 minutes
1 (id=16)  Test          2 minutes ago   7 seconds      23 seconds  23 seconds
2 (id=14)  Test          2 minutes ago   2 seconds      37 seconds  37 seconds
13         Test          1 minute ago    1 minute       2 minutes   2 minutes   True
15         Test          42 minutes ago  38 minutes     38 minutes  38 minutes  True
12         Another task  51 minutes ago  1 minute       1 minute    1 minute    True
```

### Start/stop a task by reference.

Only one task can be in progress at a time, so starting a task will stop the current task.

```shell
$ tk start 1

Stopped: Another task (ref=1)

Duration   1 minute
This week  1 minute
Total      1 minute

Started: Task name (ref=0)

This week  4 seconds
Total      4 seconds
```

### Delete a task

```shell
$ tk rm 1

Deleted: Another task (ref=1)
```

### Delete multiple tasks

```shell
$ tk rm 1 2 3

Deleted: Another task (ref=1)
Deleted: Task name (ref=2)
Deleted: Test (ref=3)
```