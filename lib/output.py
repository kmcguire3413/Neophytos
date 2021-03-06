'''
	Leonard Kevin McGuire Jr 2014 (kmcg3413@gmail.com)

	Output

	This module provides the ability for another application to monitor or
	query the running status of the program using this module. It supports
	setting global key and value pairs for the program status, and also
	key and value pairs for individual work items.

	It can dump to the standard console and TCP server, or neither. The 
	support for standard console is for programs who run this program 
	and wish to read standard output to determine the status. The TCP
	server allows this program to run stand alone and be queried by another
	process.

	Your program must add and remove work items and/or set title keys. You
	must also manage the work items being careful to remove ones that are
	not needed anymore or they will forever consume memory.
'''
import math
import os
import sys
import threading
import socket
import select
import struct
import time
import uuid
	
# globals (bad.. bad.. and bad but.. but.. and but..)
mode 		= None
title 		= {}
win 		= None
working 	= {}
__working 	= []
lock 		= threading.Lock()
server 		= None

class TCPServer:
	def __init__(self, nullsock = True):
		# try to find valid server socket
		self.socks = []
		if nullsock is False:
			self.sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
			self.sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
			bound = False
			# find a free port that we can use
			for port in range(41000, 41100):
				try:
					self.sock.bind(('localhost', port))
					# we could prolly get away with one or two, but three
					# seems quite safe considering it does not take much
					# in resources to keep a backlog of three
					self.sock.listen(3)
					bound = True
					break
				except:
					# try the next one
					continue
			# let someone know something went wrong
			if not bound:
				raise Exception('Could not find port to start TCP status server on.')
		else:
			self.sock = None
			return
		# create thread to handle connections
		thread = threading.Thread(target = TCPServer.Entry, args = (self,))
		# let thread die when primary thread dies
		thread.daemon = True
		# start the thread
		thread.start()
	
	def SendSingle(self, sock, message):
		sock.sendall(struct.pack('>I', len(message)) + bytes(message, 'utf8'))
	
	def Send(self, message):
		for sock in self.socks:
			try:
				sock.sendall(struct.pack('>I', len(message)) + bytes(message, 'utf8'))
			except Exception as e:
				# if anything happens just pretend it never existed.. LOL
				self.socks.remove(sock)
	
	'''
		This is called to bring a new connection up to date
		on the status by sending current title and current
		list of jobs and their information.
	'''
	def BringUp(self, sock):
		global working
		global title
		
		for key in title:
			self.SendSingle(sock, '[title]:%s:%s' % (key, title[key]))
		for name in working:
			work = working[name]
			self.SendSingle(sock, '[add]:%s' % name)
			if work.old:
				self.Send('[old]:%s' % name)
			for k in work.__dict__:
				self.SendSingle(sock, '[wkey]:%s:%s:%s' % (name, k, work.__dict__[k]))
			#self.SendSingle(sock, '[status]:%s:%s' % (name, work.status))
			#self.SendSingle(sock, '[progress]:%s:%s' % (name, work.progress))
		self.SendSingle(sock, '[uptodate]')

		
	'''
		The main loop that accepts connections and brings
		new connections up to date on the status.
	'''
	def Entry(self):
		global lock
		global title
	
		# if null sock just exit
		if self.sock is None:
			return

		lastdatatime = {}
	
		while True:
			# block on select
			input = [self.sock]
			# add clients
			inputs = input + self.socks
			title['sockcount'] = len(self.socks)
			# block until something happens
			readable, writable, exc = select.select(input, [], input)
			
			# remove any last data not in self.socks; inefficent but
			# i need a quick solution to solve this
			tr = []
			for sock in lastdatatime:
				if sock not in self.socks:
					tr.append(sock)
			for sock in tr:
				del lastdatatime[sock]
			
			# drop sockets with no data sent recently
			ct = time.time()
			for sock in lastdatatime:
				if ct - lastdatatime[sock] > 3:
					tr.append(sock)
					self.socks.remove(sock)
					sock.close()
			for sock in tr:
				del lastdatatime[sock]
				
			# lock so nothing tries to send stuff while we are busy
			# dropping clients or anything
			if self.sock in readable:
				# accept new connection
				readable.remove(self.sock)
				nsock, naddr = self.sock.accept()
				lastdatatime[nsock] = time.time()
				try:
					# we lock because we dont want to be trying
					# to read any lists, objects, or dicts while
					# they are being manipulated...mainly not
					# because it will corrupt state but rather
					# it could produce some weird bugs
					with lock:
						self.BringUp(nsock)
					# only add if no exceptions
					self.socks.append(nsock)
				except Exception as e:
					# just ignore it and the client sock gets silently dropped
					nsock.close()
			
			for sock in readable:
				# discard data
				data = sock.read()
				self.socks.remove(sock)
				sock.close()

			for sock in exc:
				# drop client
				self.socks.remove(sock)
				sock.close()
			continue
	
class Mode:
	StandardConsole		= 1
	TCPServer			= 2

# os.devnull
def Init(_mode):
	global win
	global mode
	global server
	
	mode = _mode
	
	if mode & Mode.TCPServer:
		# actually do the server
		server = TCPServer(nullsock = False)
	else:
		# just pretend to work
		server = TCPServer(nullsock = True)
	
	# set unique identifier for this session
	SetTitle('uid', uuid.uuid4().hex)
		
class Working:
	pass
		
def SetTitle(key, value):
	global lock
	global title
	
	with lock:
		title[key] = value
		if mode == Mode.StandardConsole:
			print('[title]:%s:%s' % (key, value))
	
def AddWorkingItem(name):
	global working
	global __working
	global lock
	global mode
	
	with lock:	
		if mode == Mode.StandardConsole:
			print('[add]:%s' % name)
		working[name] = Working()
		working[name].status = ''
		working[name].progress = 0.0
		working[name].old = False
		
		__working.insert(0, name)

		Prune()

def SetCurrentStatus(name, status):
	global working
	global lock
	
	with lock:
		if mode == Mode.StandardConsole:
			print('[status]:%s:%s' % (status, name))
		working[name].status = status
	
def SetWorkItem(name, key, value):
	global working
	global lock
	
	with lock:
		if mode == Mode.StandardConsole:
			print('[setitem]:%s:%s' % (key, value))
		working[name].__dict__[key] = value
	
def SetCurrentProgress(name, value):
	global working
	global lock
	
	with lock:
		if mode == Mode.StandardConsole:
			print('[progress]:%s:%s' % (value, name))
		working[name].progress = value
	
def Prune():
	global __working
	global working
	global mode

	max = 200
	
	while len(__working) > max:
		oldlen = len(__working)
		x = len(__working) - 1
		while x > -1:
			_name = __working[x]
			x = x - 1
			
			if working[_name].old is True:
				__working.remove(_name)
				del working[_name]
				if mode == Mode.StandardConsole:
					print('[rem]:%s' % _name)
				break
				
		if oldlen == len(__working) and len(__working) > max:
			# just let it grow.. if someone has a leak and is adding
			# and never removing then let them find out when the process
			# runs out of memory..
			# TODO: maybe add some sort of flag to periodically report through
			#       stderr a warning that something might be wrong..
			break
	
def RemWorkingItem(name):
	global working
	global __working
	global lock
	
	with lock:
		if mode == Mode.StandardConsole:
			print('[old]:%s' % name)
		working[name].old = True
		
		Prune()