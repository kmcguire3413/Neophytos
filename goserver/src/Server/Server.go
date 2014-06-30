package Server

import 			"fmt"
import tls 		"crypto/tls"
//import rsa 	"crypto/rsa"
//import crand	"crypto/rand"
import 			"net"
import			"sync"
import ioutil	"io/ioutil"
import			"bytes"
import			"strconv"
import			"os"
import pprof 	"runtime/pprof"
//import			"time"
//import			"runtime"

const (
	CmdClientDirList				= 0
	CmdClientFileRead				= 1
	CmdClientFileWrite				= 2
	CmdClientFileSize				= 3
	CmdClientFileTrun				= 4
	CmdClientFileDel				= 5
	CmdClientFileCopy				= 6
	CmdClientFileMove				= 7
	CmdClientFileHash				= 8
	CmdClientGetPublicKey			= 11
	CmdClientSetupCrypt				= 12
	CmdClientEncrypted				= 13
	CmdClientLogin					= 14
	CmdClientFileTime				= 15
	CmdClientFileSetTime			= 16
	CmdClientEcho					= 17
	////////////////////////////////////
	CmdServerDirList				= 0
	CmdServerFileRead				= 1
	CmdServerFileWrite				= 2
	CmdServerFileSize				= 3
	CmdServerFileTrun				= 4
	CmdServerFileDel				= 5
	CmdServerFileCopy				= 6
	CmdServerFileMove				= 7
	CmdServerFileHash				= 8
	CmdServerPublicKey 				= 11
	CmdServerSetupCrypt				= 12
	CmdServerEncrypted				= 13
	CmdServerLogin					= 14
	CmdServerLoginResult			= 16
	CmdServerFileTime				= 17
	CmdServerSetCompressionLevel	= 18
	CmdServerFileSetTime			= 19
	CmdServerEcho					= 20
)

// account configuration structure
type AccountConfig struct {
	lock				*sync.Mutex			// protects structure access
	spaceQuota			int64				// maximum bytes that can be used
	spaceUsed			int64				// bytes used in total file data
	spacePerFile		int64				// bytes consumed for file creation
	spacePerDir			int64				// bytes consumed for each directory created
	DiskPath			string				// the base disk path for this account
	RefCount			int16				// once at zero it can be flushed to disk
	AccountName			string				// name of account
}

// lock protected object member access; will add mod to current value and return new value
func (self *AccountConfig) SpaceQuota(mod int64) (out int64) {
	self.lock.Lock()
	self.spaceQuota += mod
	out = self.spaceQuota
	self.lock.Unlock()
	return
}

// lock protected object member access; will add mod to current value and return new value
func (self *AccountConfig) SpaceUsed(mod int64) (out int64) {
	defer self.lock.Unlock()
	self.lock.Lock()
	self.spaceUsed += mod
	return self.spaceUsed
}

// lock protected object member access; will add mod to current value and return new value
func (self *AccountConfig) SpacePerFile(mod int64) (int64) {
	defer self.lock.Unlock()
	self.lock.Lock()
	self.spacePerFile += mod
	return self.spacePerFile
}

// lock protected object member access; will add mod to current value and return new value
func (self *AccountConfig) SpacePerDir(mod int64) (int64) {
	defer self.lock.Unlock()
	self.lock.Lock()
	self.spacePerDir += mod
	return self.spacePerDir
}

// server client state object
type ServerClient struct {
	config				*AccountConfig		// pointer to account configuration
	server				*Server				// pointer to server structure
	conn 				net.Conn  			// connection object for client
	vector				uint64				// current valid vector
	msgbuf				*bytes.Buffer		// byte buffer
	msgin				[]byte				// try to reuse message buffers when possible
}

// server state object
type Server struct {
	accountConfigsLock	*sync.Mutex
	accountConfigs		map[string]*AccountConfig
}

/*
	BYTE ARRAY INTEGER READING ROUTINES
*/

// read unsigned 16-bit big endian integer
func read16MSB(buf []byte, off uint32) (uint16) {
	return uint16(buf[off + 0]) << 8 | uint16(buf[off + 1])
}

// read unsigned 32-bit big endian integer
func read32MSB(buf []byte, off uint32) (uint32) {
	return uint32(read16MSB(buf, off)) << 16 | uint32(read16MSB(buf, off + 2))
}

// read unsigned 64-bit big endian integer
func read64MSB(buf []byte, off uint32) (uint64) {
	return uint64(read32MSB(buf, off)) << 32 | uint64(read32MSB(buf, off + 4))
}

// read message from buffer and remove it from the buffer
func (self *ServerClient) getMessageFromBuffer(buf []byte, top uint32, maxsz uint32) (v uint64, msg []byte, btop uint32, err error) {
	var sz			uint32


	// read message length
	if top < (4 + 8) {
		return 0, nil, top, nil
	}
	
	sz = read32MSB(buf, 0)
	
	if sz > maxsz {
		// maximum message size prevents DoS by creating very large messages
		return 0, nil, top, fmt.Errorf("The message exceeds maximum specified size of %I.", maxsz) 
	}
	
	if (4 + 8 + sz > top) {
		// not enough data in buffer to read all of it
		return 0, nil, top, nil
	}
	
	
	v = read64MSB(buf, 4)
	
	// resize our main message buffer to hold the contents of this message
	// if it is too small or never been allocated; we do this to keep from
	// allocating new message buffers each time but the problem is you cant
	// use the message buffer across calls to this function because you will
	// get overwritten - this showed a significant decrease in cpu burn and
	// i suspect that was from the GC collecting all the short lived message
	// buffers
	if sz > uint32(len(self.msgin)) {
		fmt.Printf("expanding internal msg buffer\n")
		self.msgin = make([]byte, sz)
	}

	// get a slice of our main buffer (points into main buffer)
	msg = self.msgin[0:sz]

	// copy message into buffer
	copy(msg, buf[4 + 8:4 + 8 + sz])
	
	// shift all data down in incoming buffer
	copy(buf, buf[4 + 8 + sz:top])
	// reset buffer top
	btop = top - (4 + 8 + sz)
	return v, msg, btop, nil
}

// initializes internal message buffer and write basic fields
func (self *ServerClient) MsgStart(rvector uint64) {
	self.msgbuf.Reset()

	self.MsgWrite64MSB(self.vector)		// write server vector
	self.vector = self.vector + 1		// increment server vector
	self.MsgWrite64MSB(rvector)			// write reply vector
	/*
		I feel that I need to explain this. Back in the old days I did not
		use SSL and instead any packet that was encrypted and needed to be
		decrypted was prefixed with this. Many of the commands were required
		to be encrypted. The current build of the client does not do any 
		encrypted outside of SSL. So this is just left from those old days. I do
		not remove it because I might want employ some extra encryption one day and
		all the code is still in place (just disabled)..

		I could one day make it where commands are not forced to be encrypted and
		in that case I could leave this prefix off. -- kmcguire
	*/
	self.MsgWrite8(CmdServerEncrypted)	// prefix encrypted command (doesnt really do anything)
}

// sends message to remote
func (self *ServerClient) MsgEnd() {
	// length does not consider the two vector fields
	l := self.msgbuf.Len() - (8 * 2)
	out := self.msgbuf.Bytes()

	hdr := []byte {byte(l >> 24), byte(l >> 16), byte(l >> 8), byte(l)} 

	self.conn.Write(hdr)
	self.conn.Write(out)
}

func (self *ServerClient) MsgWrite8(v uint8) {
	self.msgbuf.Write([]byte {v})
}

func (self *ServerClient) MsgWrite16MSB(v uint16) {
	self.msgbuf.Write([]byte {byte((v >> 8) & 0xff), byte(v & 0xff)})
}

func (self *ServerClient) MsgWrite32MSB(v uint32) {
	self.msgbuf.Write([]byte {byte(v >> 24 & 0xff), byte(v >> 16 & 0xff), byte(v >> 8 & 0xff), byte(v & 0xff)})
}

func (self *ServerClient) MsgWrite64MSB(v uint64) {
	self.MsgWrite32MSB(uint32(v >> 32))
	self.MsgWrite32MSB(uint32(v & 0xffffffff))
}

func (self *ServerClient) MsgWrite(b []byte) {
	self.msgbuf.Write(b)
}

func (self *ServerClient) MsgWriteString(b string) {
	self.msgbuf.WriteString(b)
}

// process a message by executing what it commands under the ServerClient context
func (self *ServerClient) ProcessMessage(vector uint64, msg []byte) (err error) {
	var cmd			byte

	cmd = msg[0]
	
	// self.LoadAccountConfig("ok45JeXm3")

	switch (cmd) {
		case 11: // GetPublicKey
			panic("not impemented: GetPublicKey")
		case 12: // SetupCrypt
			panic("not impemented: SetupCrypt")
	}
	
	// encrypted check (must be of type encrypted)
	if cmd != 13 {
		// bad message obviously
		return fmt.Errorf("unknown message type (looking for encrypted)")
	}
	
	// get sub-type
	cmd = msg[1]
	msg = msg[2:]

	// check if login mesage
	if cmd == CmdClientLogin {
		// load account configuration
		self.config = self.server.LoadAccountConfig(string(msg))
		fmt.Printf("config:%p account:%s\n", self.config, string(msg))
		self.MsgStart(vector)
		self.MsgWrite8(CmdServerLoginResult)
		if self.config == nil {
			self.MsgWrite8('n')
			return
		}
		self.MsgWrite8('y')
		self.MsgEnd()
	}

	if self.config == nil {
		// oops.. either no login command issues or account was invalid
		// because we were unable to load the file or any other issue so
		// go ahead and terminate the connection here
		panic("client configuration not loaded")
	}

	switch (cmd) {
		case CmdClientDirList:
			path := fmt.Sprintf("%s/%s", self.config.DiskPath, string(msg))
			//fmt.Printf("DirList:%s\n", path)
			nodes, err := ioutil.ReadDir(path)

			self.MsgStart(vector)
			self.MsgWrite8(CmdServerDirList)
			if err != nil {
				// could not access directory because it does not exist.. or other things..
				self.MsgWrite8(0)
				self.MsgEnd()
				break
			}

			self.MsgWrite8(1)
			for _, n := range nodes {
				// write length of name
				self.MsgWrite16MSB(uint16(len(n.Name())))
				// write if directory or not
				if n.IsDir() {
					self.MsgWrite8(1)
				} else {
					self.MsgWrite8(0)
				}
				// write name
				//self.MsgWrite([]byte(n.Name()))
				self.MsgWriteString(n.Name())
			}
			// send message to remote
			self.MsgEnd()
		case CmdClientEcho:
			self.MsgStart(vector)
			self.MsgWrite8(CmdServerEcho)
			self.MsgEnd()
		case CmdClientFileRead:
		case CmdClientFileWrite:
		case CmdClientFileSize:
			path := fmt.Sprintf("%s/%s", self.config.DiskPath, string(msg))
			stat, err := os.Stat(path)
			if err != nil || stat == nil {
				// path is not accessible for whatever reason
				break
			}

		case CmdClientFileTrun:
		case CmdClientFileDel:
		case CmdClientFileCopy:
		case CmdClientFileMove:
		case CmdClientFileHash:
		case CmdClientFileTime: 
		case CmdClientFileSetTime: 
	}
	
	return nil
}

func (self *ServerClient) Finalize() {
	// decrement the ref count on account configuration object
	// so that it can be unload and dumped to disk
	if self.config != nil {
		self.server.FinalizeAccountConfig(self.config)
	}
}

// handles a single client connection
func (self *ServerClient) ClientEntry(conn net.Conn) {
	var buf			[]byte
	var btop		uint32
	var err			error
	var count		int
	var vector		uint64
	var msg			[]byte
	const maxmsgsz  uint32 = 1024 * 1024 * 4
	
	defer self.Finalize()

	f, _ := os.Create("prof")
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	self.conn = conn

	// message buffer
	buf = make([]byte, 1024 * 1024 * 4)
	btop = 0

	// loop
	for {
		// read data from connection
		count, err = conn.Read(buf[btop:])
		if count == 0 {
			// connection is dropped (just exit)
			fmt.Printf("client connection dropped")
			return
		}
		
		btop = btop + uint32(count)
		
		// message fetch loop
		for vector, msg, btop, err = self.getMessageFromBuffer(buf, btop, maxmsgsz); 
			msg != nil && err == nil; 
			vector, msg, btop, err = self.getMessageFromBuffer(buf, btop, maxmsgsz) {
			// process message
			self.ProcessMessage(vector, msg)
		}
	}
	return
}

// create a ServerClient object
func (self *Server) NewClient() (sc *ServerClient) {
	sc = new(ServerClient)
	sc.server = self
	sc.vector = 0
	sc.msgbuf = new(bytes.Buffer)
	return
}

// called by ServerClient to load/get account configuration
func (self *Server) LoadAccountConfig(account string) (*AccountConfig) {
	// check if it is already loaded
	//ioutil.ReadFile(filename) ([]byte, error)
	var config			*AccountConfig

	fmt.Printf("@@@@@@@@@@@@@@\n")

	// make sure it is unlocked on function exit or panic
	defer self.accountConfigsLock.Unlock()

	// try to lock
	self.accountConfigsLock.Lock()
	
	if _, ok := self.accountConfigs[account]; !ok {
		// load configuration from disk if it can be found
		data, err := ioutil.ReadFile(fmt.Sprintf("./accounts/%s", account))
		if err != nil {
			// an error occured trying to load the config
			return nil
		}
		// break data into lines and then parse each line
		lines := bytes.Split(data, []byte {10})

		config = new(AccountConfig)

		config.lock = &sync.Mutex{}
		config.AccountName = account
		
		for index := range lines {
			// trip off white space and line endings
			line := bytes.Trim(lines[index], " 	\n\r")
			fmt.Printf("line:%s\n", line)
			// break into key and value pair
			colon := bytes.IndexByte(line, 58)
			if colon < 0 {
				continue
			}
			key := string(bytes.Trim(line[0:colon], " 	\n\r"))
			val := string(bytes.Trim(line[colon + 1:], " 	\n\r"))
			// store value properly.. if known
			switch (key) {
				case "DiskPath":
					config.DiskPath = val
				case "SpaceQuota":
					nval, err := strconv.Atoi(val)
					if err != nil {
						fmt.Printf("value %s for account config key [%s] is not an integer!\n", val, key)
					} else {
						config.SpaceQuota(int64(nval))
					}
				case "SpaceUsed":
					nval, err := strconv.Atoi(val)
					if err != nil {
						fmt.Printf("value [%s] for account config key [%s] is not an integer!\n", val, key)
					} else {
						config.SpaceUsed(int64(nval))
					}
				case "SpacePerFile":
					nval, err := strconv.Atoi(val)
					if err != nil {
						fmt.Printf("value [%s] for account config key [%s] is not an integer!\n", val, key)
					} else {					
						config.SpacePerFile(int64(nval))
					}
				case "SpacePerDir":
					nval, err := strconv.Atoi(val)
					if err != nil {
						fmt.Printf("value [%s] for account config key [%s] is not an integer!\n", val, key)
					} else {
						config.SpacePerDir(int64(nval))
					}
				default:
					fmt.Printf("unknown key in account configurtion as [%s]\n", key)
			}
		}

		self.accountConfigs[account] = config
	} else {
		config = self.accountConfigs[account]
	}
	
	config.RefCount++

	return config
}

// writes account configuration to disk
func (self *Server) FlushAccountConfig(config *AccountConfig) {
	// convert members to string
	outstr := fmt.Sprintf("SpaceQuota: %d\nSpaceUsed: %d\nSpacePerFile: %d\nSpacePerDir: %d\nDiskPath: %s\n", config.SpaceQuota(0), config.SpaceUsed(0), config.SpacePerFile(0), config.SpacePerDir(0), config.DiskPath)
	// convert to bytes
	out := []byte(outstr)

	// write to disk
	ioutil.WriteFile(fmt.Sprintf("./accounts/%s", config.AccountName), out, 0700)
}

// decrement reference count and flush to disk if no more references
func (self *Server) FinalizeAccountConfig(config *AccountConfig) {
	// make sure it is unlocked on function exit or panic
	defer self.accountConfigsLock.Unlock()

	// try to lock
	self.accountConfigsLock.Lock()

	config.RefCount--

	// if no more references then flush to disk and set to be collected
	if config.RefCount < 1 {
		// remove from map
		delete(self.accountConfigs, config.AccountName)
		// flush to disk
		self.FlushAccountConfig(config)
	}
}

// starts a server and will not return
func (self *Server) ServerEntry(psignal chan uint32) {
	var sc				*ServerClient
	var cert			tls.Certificate
	var config			tls.Config
	
	fmt.Println("Server Started")
	
	// signal caller we are ending
	defer func () { psignal <- 0 } ()

	//func (self *Server) LoadAccountConfig(account string) (*AccountConfig) {

	self.accountConfigsLock = &sync.Mutex{}
	self.accountConfigs = make(map[string]*AccountConfig)
	self.LoadAccountConfig("pl45JeXm3")
	self.LoadAccountConfig("xp45JeXm3")

	fmt.Printf("loading certificate\n")
	cert, err := tls.LoadX509KeyPair("cert.pem", "cert.pem")
	fmt.Printf("creating config\n")
	config = tls.Config{Certificates: []tls.Certificate {cert}, InsecureSkipVerify: true}
	
	config.ServerName = "kmcg3413.net"
	
	// create a listening socket
	fmt.Printf("creating listener\n");
	ln, err := tls.Listen("tcp", ":4323", &config)
	
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	
	if ln == nil {
		fmt.Println("There was an error creating the NET/TLS listener.")
		return
	}
	
	fmt.Printf("ready for connections\n")
	// handle connections
	for {
		conn, err := ln.Accept()
		
		if err != nil {
			fmt.Printf("accept-error: %s\n", err)
			continue
		}
		
		if conn == nil {
			fmt.Printf("accept-error: %s\n", err)
			continue
		}
		
		fmt.Printf("new client accepted\n")
		sc = self.NewClient()
		go sc.ClientEntry(conn)
	}

	return
}