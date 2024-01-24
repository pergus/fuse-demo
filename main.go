package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"syscall"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	_ "bazil.org/fuse/fs/fstestutil"
	"bazil.org/fuse/fuseutil"
)

/*
var _ fs.FS = (*FS)(nil)
var _ fs.NodeStringLookuper = (*Node)(nil)
var _ fs.HandleReadDirAller = (*Node)(nil)
var _ fs.Node = (*Node)(nil)
var _ fs.NodeOpener = (*Node)(nil)
var _ fs.Handle = (*Node)(nil)
var _ fs.HandleReader = (*Node)(nil)
*/

// FS is the filesystems
type FS struct {
	Nodes map[string]*Node
}

// Node defines a directory or file
type Node struct {
	fs      *FS
	Inode   uint64
	Name    string
	Type    fuse.DirentType
	Content []byte
}

type file struct {
	name    string
	content string
}

type directory struct {
	name  string
	files []file
}

var dirs = []directory{

	directory{name: "cat", files: []file{
		file{name: "file1.txt", content: "This is file 1 + some random stuff..."},
		file{name: "file2.txt", content: "This is file 2 + random stuff"},
		file{name: "file3.txt", content: "This is file 3 + tail of characters to make the size unique"},
		file{name: "file4.txt", content: "This is file 4 + Nothing to see here"},
		file{name: "file5.txt", content: "This is file 5 + Usage"}}},

	directory{name: "indices", files: []file{
		file{name: "file6.txt", content: "This is file 6 + some random stuff..."},
		file{name: "file7.txt", content: "This is file 7 + random stuff"},
		file{name: "file8.txt", content: "This is file 8 + tail of characters to make the size unique"},
		file{name: "file9.txt", content: "This is file 9 + Nothing to see here"},
		file{name: "file10.txt", content: "This is file 10 + Usage"}}},

	directory{name: "shards", files: []file{
		file{name: "file11.txt", content: "This is file 11 + some random stuff..."},
		file{name: "file12.txt", content: "This is file 12 + random stuff"},
		file{name: "file13.txt", content: "This is file 13 + tail of characters to make the size unique"},
		file{name: "file14.txt", content: "This is file 14 + Nothing to see here"},
		file{name: "file15.txt", content: "This is file 15 + Usage"}}},
}

// Root return root directory
func (f *FS) Root() (fs.Node, error) {
	return &Node{fs: f}, nil
}

// Lookup a file or directory
func (n *Node) Lookup(ctx context.Context, name string) (fs.Node, error) {
	node, ok := n.fs.Nodes[name]
	if ok {
		return node, nil
	}
	return nil, fuse.ENOENT
}

// ReadDirAll reads all content of a directory
func (n *Node) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	var dirDirs []fuse.Dirent
	for _, node := range n.fs.Nodes {
		dirent := fuse.Dirent{
			Inode: node.Inode,
			Name:  node.Name,
			Type:  node.Type,
		}
		dirDirs = append(dirDirs, dirent)
	}
	return dirDirs, nil
}

// Attr sets the attributes of a file
func (n *Node) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeDir | 0555
	if n.Type == fuse.DT_File {
		a.Inode = n.Inode
		a.Mode = 0444
		a.Size = uint64(len(n.Content))
	}
	if a.Inode == 0 {
		a.Inode = 1
	}
	return nil
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s MOUNTPOINT\n", os.Args[0])
	flag.PrintDefaults()
}

func close(c io.Closer) {
	err := c.Close()
	if err != nil {
		panic(err)
	}
}

// Open a file
func (n *Node) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	if !req.Flags.IsReadOnly() {
		return nil, fuse.Errno(syscall.EACCES)
	}
	resp.Flags |= fuse.OpenDirectIO
	return n, nil
}

// Read the content of a file
func (n *Node) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	fmt.Printf("Reading file %q from %v to %v\n", n.Name, req.Offset, req.Size)
	fuseutil.HandleRead(req, resp, n.Content)
	return nil
}

func mount(mountpoint string) (err error) {
	c, err := fuse.Mount(
		mountpoint,
		fuse.FSName("eFS"),
		fuse.Subtype("efs"),
		fuse.LocalVolume(),
		fuse.VolumeName("E filesystem"),
	)
	if err != nil {
		return
	}
	defer close(c)

	if p := c.Protocol(); !p.HasInvalidate() {
		return fmt.Errorf("kernel FUSE support is too old to have invalidations: version %v", p)
	}

	srv := fs.New(c, nil)
	filesys := &FS{}
	filesys.Nodes = make(map[string]*Node, 1)

	// Add directories and their files to the nodes map.
	for i, dir := range dirs {
		filesys.Nodes[dir.name] = &Node{Name: dir.name, Inode: uint64(i), Type: fuse.DT_Dir, fs: &FS{}}
		filesys.Nodes[dir.name].fs.Nodes = make(map[string]*Node, 1)
		for _, f := range dir.files {
			filesys.Nodes[dir.name].fs.Nodes[f.name] = &Node{
				Name:    f.name,
				Inode:   fs.GenerateDynamicInode(uint64(i), f.name),
				Type:    fuse.DT_File,
				Content: []byte(f.content)}
		}
	}
	err = srv.Serve(filesys)
	if err != nil {
		return
	}

	// Check if the mount process has an error to report.
	<-c.Ready
	err = c.MountError
	return
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() != 1 {
		usage()
		os.Exit(2)
	}
	mountpoint := flag.Arg(0)

	err := mount(mountpoint)
	if err != nil {
		panic(err)
	}
}
