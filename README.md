# shortenfs

Shortenfs is an experiment in storing data where one is generally not supposed to store data - namely, in someone else's URL shortener.

Upon mounting, a FUSE-backed block device is presented, which can be formatted with whatever filesystem(s) the user desires.

*This is not intended to be used for serious applications, and has not been tuned for performance.*

# Installation

```shell script
go install github.com/1ttric/shortenfs
```

# Usage

Since most URL shorteners are write-only, the 'root' shortlink ID for the virtual block device must be updated after each write.

The config file will therefore be overwritten with the new root ID upon exit.

First, create an appropriate config file in YAML format.

```yaml
# The URL shortener driver to use for this mount
driver: tinyurl
# The driver-specific shortlink ID to use for this mount. At first this will be empty, but will be updated upon unmount.
rootid: ""
# The depth of the node tree - a larger depth increases exponentially both the storage available but also the time required to perform a read or write
depth: 1
# Driver-specific options (refer to driver documentation)
driveropts: null
``` 

Then, mount the FUSE layer into a directory. This exposes a block device.

```
will@laptopalfa shortenfs > go run main.go mount -c config.yml /tmp/mount 
INFO[0000] using config file config.yml                 
INFO[0000] mounted filesystem

will@laptopalfa shortenfs > ls -alh /tmp/mount
total 0
-rwxrwxrwx 1 root root 4.0M Oct 18 16:39 block
```

As an example, let's create an ext4 filesystem on the block device. This will take a minute or two.

```
will@laptopalfa shortenfs > mkfs.ext4 /tmp/mount/block
mke2fs 1.45.6 (20-Mar-2020)
Creating filesystem with 4028 1k blocks and 1008 inodes

Allocating group tables: done                            
Writing inode tables: done                            
Creating journal (1024 blocks): done
Writing superblocks and filesystem accounting information: done

```

... and mount it

```
will@laptopalfa shortenfs > losetup -fP /tmp/mount/block
losetup: /tmp/mount/block: Warning: file does not fit into a 512-byte sector; the end of the file will be ignored.
will@laptopalfa shortenfs > sudo mount /dev/loop4 /mnt/shortenfs
root@laptopalfa shortenfs > ls -al /mnt/shortenfs
total 17
drwxr-xr-x 3 root root  1024 Oct 18 16:49 .
drwxr-xr-x 7 root root  4096 Oct 18 16:42 ..
drwx------ 2 root root 12288 Oct 18 16:49 lost+found
```

Now you can store whatever data you want!

Here's a small filesystem with some data you can look at: tinyurl/y5qne2p9