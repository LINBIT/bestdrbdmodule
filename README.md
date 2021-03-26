# bestdrbdmodule

A web service that receives /etc/os-release and uname -r and returns the best matching DRBD kernel module

# usage

```
cat /etc/os-release | curl -T - -X POST localhost:8080/api/v1/best/3.10.0-957.27.2.el7.x86_64 -s
```
