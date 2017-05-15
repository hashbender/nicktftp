# nicktftp


I started by writing the pareBytes function.  This allowed me to get more familiar with the protocol and build an understanding of lower-level operations.  As I wrote this function I realized the need for 2 structs, the Request and Response.

After writing parseBytes, I started working on the file indexing.  I tested this by writing the get/puts cases without actually doing the read/write.  i.e. just making sure that the message from the client resulted in the appropriate response

**Running**

*locally*

Assuming you have a Go environment set up, you should just be able to run from the top director

`go install . && nicktftp`

*Docker*

I've provided a docker file.  You can run 

`docker build` 

and

`docker run`


***note*** 

If running on Mac, you'll need to expose some ports:


    for i in {10000..10999}; do
        VBoxManage modifyvm "boot2docker-vm" --natpf1 "tcp-port$i,tcp,,$i,,$i";
        VBoxManage modifyvm "boot2docker-vm" --natpf1 "udp-port$i,udp,,$i,,$i";
    done




**Issues**

Not sure what to do about overwriting files.  Looking around online, it seems that it's up to the file permissions to determine if the server will overwrite the file.  I've written the server such that files cannot be overwritten unless the original transfer attempt failed, then the client can retry

Error handling: I've written in basic error handling per the spec, but more robust error handling would be needed for a production solution.  There's no alerting or performance monitoring. 

### References:
https://tools.ietf.org/html/rfc1350

http://www.minaandrawos.com/2016/05/14/udp-vs-tcp-in-golang/