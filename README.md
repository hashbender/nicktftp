# nicktftp
This is a basic tftp server which holds files in memory.  


## Methodology

I started by writing the request parsing function.  This allowed me to get more familiar with the protocol and build an understanding of lower-level operations.  As I wrote this function I realized the need for a struct to encapsulate the results.

After writing NewRequest, I started working on the file indexing.  I tested this by writing the get/puts cases without actually doing the read/write.  i.e. just making sure that the message from the client resulted in the appropriate response.


## Running

**locally**

Assuming you have a Go environment set up, you should just be able to run from the top director

`go install . && nicktftp`

**Docker**

I've provided a docker file.  You can run 

`docker build` 

and

`docker run`


*NOTE* 

If running on Mac, you'll need to expose some ports:


    for i in {10000..10999}; do
        VBoxManage modifyvm "boot2docker-vm" --natpf1 "tcp-port$i,tcp,,$i,,$i";
        VBoxManage modifyvm "boot2docker-vm" --natpf1 "udp-port$i,udp,,$i,,$i";
    done




## Issues

Not sure what to do about overwriting files.  Looking around online, it seems that it's up to the file permissions to determine if the server will overwrite the file.  I've written the server such that files cannot be overwritten unless the original transfer attempt failed, then the client can retry

Error handling: I've written in basic error handling per the spec, but more robust error handling would be needed for a production solution.  There's no alerting or performance monitoring. 

## Testing

This solution has minimal automated testing.  Before considering this code production ready, I would require a suite of unit tests as well as a few targeted end-to-end integration tests to catch regressions. 


## References:
https://tools.ietf.org/html/rfc1350

http://www.minaandrawos.com/2016/05/14/udp-vs-tcp-in-golang/
