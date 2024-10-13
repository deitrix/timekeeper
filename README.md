# Timekeeper

## Installation

```shell
go install github.com/deitrix/timekeeper/cmd/tk@latest
```

## How to use

Start tracking a new task

```shell
$ tk start "Task name"

Started: Task name (ref=0)
```

Stop tracking the current task

```shell
$ tk stop

Stopped: Task name (ref=0)
    
Duration   20 seconds
This week  20 seconds
Total      20 seconds
```

Restart the last task (reference 0)

```shell
$ tk start

Started: Another task (ref=0)
```

View the current task (affected by start and stop)

```shell
$ tk

In progress: Another task (ref=0)

Duration   2 seconds
This week  2 seconds
Total      2 seconds
```

List tasks.

Each task has a reference number that can be used to perform actions on that task. For the first 10
tasks, the reference number is the same as the index in the list, to make it easier to reference
recent tasks. After 10 tasks, the reference number is a unique identifier which is generated on task
creation.

```shell
$ tk ls

Ref  Name          Last Start      Last Duration  This Week   Total
0    Another task  55 seconds ago  55 seconds     55 seconds  55 seconds
1    Task name     1 minute ago    4 seconds      4 seconds   4 seconds
```

Start/stop a task by reference. Only one task can be in progress at a time, so starting a task will stop the current task.

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