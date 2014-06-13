![neophytos-logo](http://kmcg3413.net/logo.png)
=====

The Neophytos system has the following features:
* secure communication from client to server
* multiple backup targets per account
* multiple sub-accounts with different permissions if desired (read/write)
* delta uploading/downloading (only uploads/downloads changed portion of file)
* file stashing (supports versions and recovery of deleted files if desired)
* open source client and server
* cross-platform (Linux, Windows, ARM, or anywhere Python 3.x will run)
* files can be encrypted client side (in memory) before being sent (sensitive data protected on server)
* service provided for customers (currently in development)
* filter per target using regular expression, file size checks, and mode checks (exclusion and inclusion rule support)

The communication from the client to server by default uses SSL which is same technology
used over HTTPS. There is also a built in much less secure encryption that can maximize your bandwidth when uploading or downloading data. Even though the client and server is open source you will still need access to a server in order to incorporate your own encryption. You can however implement your own encryption for the actual file data that is stored on the server thus encrypting your file data over the internet.

Each account can support an unlimited number of targets on the server. This allows you to say backup your documents to the _documents_ target on the server, and also backup your work files to _work_. The target is basically like a sub-directory on the server. Also account can be created that reference into other accounts so that you could create an additional account that references into a sub-folder called _presentations_ in your _documents_ target that only has read permission. This allows you to provide access to other users with out allowing them to write to certain files or see certain files.

The client and server currently support a primitive delta patching algorithm that only transmits changes between files which further reduces the time and bandwidth needed when backing up or restoring data. The
algorithm is designed so that most of it resides client side so you have a wide range of opportunities for
improving or tailoring it to your specific situations.

The file stashing allows you to store the same file multiple times on the server. This allows you to store different versions. Perhaps you may wish to keep the original copy of a file, or always keep the most recent version from backup to backup. It can even allow you to keep a copy around of a deleted file. You can have an unlimited number of versions however they all consume space on the server.

Because the client and server are open source you can change it to suit your needs, add features, or change existing features. It also allows you a more secure installation on your machine because you can verify what the client does, has access too, and what is transmitted.

The client and server are cross-platform. They will run anywhere that Python 3.x will run. This allows you to have clients on various platforms using the same data. It also allows the server to run on any system although it has been tailored to Linux. There can be some problems for example between Windows and Linux with case-sensitive files where Linux supports case-sensitive file names and Windows does not. So the server is tailored for a Linux platform, but it can run on any platform and support could be easily added for Windows to become case-sensitive by encoding the file names where needed.

Your files can be encrypted locally (client side) before being transmitted. Your actual file on your disk is not encrypted but rather it is encrypted as it is transmitted, and decrypted as it is received from the server and restored. This ensures highly sensitive data is not vulnerable server side in the event someone was able to access the data. This is completely client implemented allowing complete control.

_Currently, the customer service is in development along with the software._ For those of you who do not wish, need, or have the capability to run your own server a service is provided for a charge that is competitive with current backup solutions to store your data.

Current State
=====
The software is currently in development. It is not officially released as stable. It is not completely usable because it does lack some features described above. It however is very close to being finished. This is a list of things and their progress.

* client command line interface (85% complete; _usable_)
* file stashing (50% complete; implemented server side but not used client side)
* file encryption (0% complete; implemented server side but client does not provide this functionality yet)
* encrypted communication between client and server over the internet (SSL and XORMIX) (100% complete)
* delta uploading/downloading (100% complete; client side needs more testing and tweaking)
* graphical front-end (0% complete; will provide a graphical front end for X11 and Windows)
* filter system (100% complete; might need reworking to be easier to use..)

Client Tutorial
=====
To run the client use:

    python3 backup.py
	
This should display a help screen. You will first need to create an account configuration. This does _NOT_ create an account on the server
but rather creates the configuration for the account.

    python3 backup.py add default localhost

Now, we need to add an authorization code (username/password combination). We will use the code created in the server tutorial (next section).

    python3 backup.py default config hdk392Ej  

That code is your username and password. It should be a lot longer! But, we are using a short one for an example.

Now, we need to add a path to backup:

    python3 backup.py default add home /home/kmcguire
	
I created a target called _home_ under the account called _default_ using the path _/home/kmcguire_.

To push files to the server use:

    python3 backup.py default push
	
This will push (backup) the files specified in the path under each target in the default account. For example:

    python3 backup.py default add projects /home/projects
	
This would add another target. So when you did push it would push both targets. To push individual targets you can use:

    python3 backup.py default push projects <... another ...>

To set rules with regular expressions on what files to push you can do:

    python3 backup.py default filter home add true repattern "^\."
	
This will cause all hidden files and directories (under Linux) or any file which starts with a period to be omitted from the backup. The
filter is checked against directory names and file names. It only uses the directory name or file name not the entire absolute path when
checking for matches. The _true_ means this is an inversion filter meaning when it matches it excludes the file or directory. You could
do the following to include only hidden directories and files.

    python3 backup.py default filter home clear
    python3 backup.py default filter home add false repattern "^\."       <--- accepts anything starting with .
	python3 backup.py default filter home add true repattern "^."         <--- rejects anything
	
You actually do not need the last command as a filter by default if it has no matches will reject a directory or file. I just
added the last line for more clarity. It is actually redundant as it would have been rejected anyway. The filter entries are
appended to the end of the list. The filter list is moved through starting from the first.

By doing:

    python3 backup.py default filter home list

You will see the filter items listed. The top one at _00_ is executed first. If it matches then it does not check the
rest of the entries. If it does not match it continues to the next item _01_ and so forth. If it runs out of entries it
will by default reject/ignore the directory or file. 

There may be a situation where you add a filter item and you need to move it to a different position on the list. There are
multiple ways to move it. For example if you needed to move it to the top of the list and when you did the add it was placed
at index _09_. To move it to the top you can do:

    python3 backup.py default filter home move 9 0

This would move entry at index _09_ to index _00_ which would push everything down. Now what was at _00_ is at _01_ and what
was at _01_ is at _02_ and so forth. 

You can also swap by doing:

    python3 backup.py default filter home move swap 9 0
	
This would swap index _09_ with index _00_. This is likely to be useful when you wish to replace something as you can then do:

    python3 backup.py default filter home del 9
	
Which would delete the old entry you swapped it out for. To find out all the commands you can do:

    python3 backup.py default filter
    python3 backup.py default filter home add
    python3 backup.py default filter home move

Just replace _home_ with your target if you use something different. The _move_ command actually supports quite a few variations
such as:

    python3 backup.py default filter home move 4 +1    <-- move index 4 up one spot
    python3 backup.py default filter home move 4 +3    <-- move index 4 up three spots
    python3 backup.py default filter home move 4 -3    <-- move index 4 down three spots
    python3 backup.py default filter home move 4 down  <-- move index 4 down one spot
    python3 backup.py default filter home move 4 up    <-- move index 4 up one spot
    python3 backup.py default filter home move 4 1     <-- move index 4 to index 1

And, remember to see your filter list use:

    python3 backup.py default filter home list
    python3 backup.py default filter projects list
    python3 backup.py default filter <target-name-here> list
	
Server Tutorial
=====

First we need to generate a certificate and an RSA private/public key pair. The certificate is used with SSL and the private/public RSA key pair are used by both SSL and other supported communication link encryptions.

    openssl req -new -x509 -days 365 -nodes -out cert.pem -keyout cert.pem --newkey rsa:8192

_This will generate a 8192-bit key pair which should be decently secure. You should be able to generate and use any size, but
it will cause connection setup times to be longer and effectively pause your server during these events. So if your dealing
with say 100 clients and it takes like 4 seconds to setup the connection then your looking at 400 seconds of pause time at
worst if the encryption/decryption is done in pure Python. If your using the SSL (by default) then it is likely a lot better
but if your using XORMIX (default if your disable SSL) then you could be looking at 2 - 4 seconds._

_The SSL may support encrypted certification or private keys, but XORMIX does not. All connections over XORMIX will fail, however
this may be desired and can effectively disable it, but I would rather implement a better ability to disable anything other than
SSL._
	
This file should be named _cert.pem_ in the server's working directory. The server can now be started if desired, or you can wait.

The next step is creating an account. There should be a sub-directory created called _accounts_ in the server's working directory. Inside create a file named exactly _hdk392Ej_. This will be the authorization code used to access this account. The _accounts_ directory should be kept **SECRET**. Anyone with access to it will know the authorization code for any account. I suspect if they have access to this directory then they likely have access to any data for the accounts. We will discuss data in a moment.

Once the file _hdk392Ej_ is created open it with an editor. Place these contents inside:

    {
     'disk-path': '/home/kmcguire/data/ok493L3Dx92Xs029W',
     'disk-quota': 1024 * 1024 * 50,
     'disk-used': 0,
     'disk-used-perfile': 4096
    }

The `1024 * 1024 * 50` equates to 50MB. Once the server loads this account and performs any operations under the account it may be replaced with a single integer value. Maybe in later versions of the server it will be replaced with a more easily human readable form. The _disk-used_ represents the bytes used by the account. The _disk-used-perfile_ represents the bytes to subtract for each file created and tries to sort of serve to account for an account with lots of small or empty files.

The server has now been configured and setup for the account _hdk392Ej_. The _hdk392Ej_ serves as the username and password combination. I recommend using something much longer. If you are intent on using something resembling more of a username and password combination then you could do something along the lines of _username.password_ and replace each with the respective username and password.

_I would live to add support in later for a more securish (at least looking) account database. At the moment the server and client are still in development and this is not by any means the final form but simply represents a roughed in system._
