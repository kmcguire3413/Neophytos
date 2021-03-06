import socket
import struct
import random
import timeit
import threading
import os
import hashlib
import time
import sys
from ctypes import *
import platform

def setProcessPriorityIdle():
	""" Set The Priority of a Windows Process.  Priority is a value between 0-5 where
		2 is normal priority.  Default sets the priority of the current
		python process but can take any valid process ID. """
	
	if os.name.lower().find('nt') != 0:
		os.nice(19)
		return
	try:
		import win32api, win32process, win32con
		
		priorityclasses = [	win32process.IDLE_PRIORITY_CLASS,
							win32process.BELOW_NORMAL_PRIORITY_CLASS,
							win32process.NORMAL_PRIORITY_CLASS,
							win32process.ABOVE_NORMAL_PRIORITY_CLASS,
							win32process.HIGH_PRIORITY_CLASS,
							win32process.REALTIME_PRIORITY_CLASS]
							
		pid = win32api.GetCurrentProcessId()
		handle = win32api.OpenProcess(win32con.PROCESS_ALL_ACCESS, True, pid)
		win32process.SetPriorityClass(handle, priorityclasses[0])
	except ImportError:
		pass

class SymCrypt:
	def __init__(self, key):
		self.xkey = key[0:len(key) >> 1]
		self.mkey = key[len(key) >> 1:]
		
		# try to load native support for encryption/decryption
		try:
			bits, ostype = platform.architecture()
			if ostype.lower().startswith('windowspe'):
				if bits.startswidth('32'):
					# 32-bit
					print('using 32-bit PE DLL')
					self.so = cdll.LoadLibrary('./native/native32.dll')
					self.so_crypt = CFUNCTYPE(c_int)(('crypt', self.so))
					self.so_decrypt = CFUNCTYPE(c_int)(('decrypt', self.so))
				else:
					# 64-bit
					print('using 64-bit PE DLL')
					self.so = cdll.LoadLibrary('./native/native64.dll')
					self.so_crypt = CFUNCTYPE(c_int)(('crypt', self.so))
					self.so_decrypt = CFUNCTYPE(c_int)(('decrypt', self.so))
			if ostype.lower().startswith('elf'):
				if bits.startswith('32'):
					print('using 32-bit shared object')
					self.so = cdll.LoadLibrary('./native/native32.so')
					self.so_crypt = CFUNCTYPE(c_int)(('crypt', self.so))
					self.so_decrypt = CFUNCTYPE(c_int)(('decrypt', self.so))
				else:
					print('using 64-bit shared object')
					self.so = cdll.LoadLibrary('./native/native64.so')
					self.so_crypt = CFUNCTYPE(c_int)(('crypt', self.so))
					self.so_decrypt = CFUNCTYPE(c_int)(('decrypt', self.so))
		except OSError:
			# well.. we tried.. fallback to Python code (SLOW..)
			self.so_crypt = None
			self.so_decrypt = None
		
		#data = b'hello world from python to C'
		# int crypt(uint8 *xkey, int xkeysz, uint8 *mkey,  int mkeysz, uint8 *data, int dsz) {
		#self.so_crypt(c_char_p(self.xkey), c_int(len(self.xkey)), c_char_p(self.mkey), c_int(len(self.mkey)), c_char_p(data), c_int(len(data)))
		#data = self.crypt(data)
		#self.so_decrypt(c_char_p(self.xkey), c_int(len(self.xkey)), c_char_p(self.mkey), c_int(len(self.mkey)), c_char_p(data), c_int(len(data)))
		#data = self.decrypt(data)
		#print('@@', self.xkey)
		#print('##', data.decode('utf8', 'ignore'))
		#exit()
		
	def __both(self, data):
		di = 0
		ki = 0
		key = self.xkey
		out = []
		while di < len(data):
			out.append(data[di] ^ key[ki])
			di = di + 1
			ki = ki + 1
			if ki >= len(key):
				ki = 0
		return bytes(out)
		
	def mix(self, data):
		data = bytearray(data)
	
		dl = len(data)
		key = self.mkey
		
		di = 0
		ki = 0
		while di < dl:
			b = data[di]
			
			kv = key[ki]
			if kv == 0:
				kv = 1
			tondx =  (dl - 1) % kv
			
			data[di] = data[tondx]
			data[tondx] = b
			
			di = di + 1
			ki = ki + 1
			if ki >= len(key):
				ki = 0
		return bytes(data)
		
	def unmix(self,  data):
		data = bytearray(data)
		dl = len(data)
		key = self.mkey

		mix = []
		# generate the sequence so that
		# i can play it backwards
		di = 0
		ki = 0
		while di < dl:
			kv = key[ki]
			if kv == 0:
				kv = 1
			tondx = (dl - 1) % kv
			mix.append((di, tondx))
			di = di + 1
			ki = ki + 1
			if ki >= len(key):
				ki = 0

		ml = len(mix)
		mi = ml - 1
		
		while mi > -1:
			frmndx = mix[mi][0]
			tondx = mix[mi][1]
		
			a = data[tondx]
			b = data[frmndx]
			
			data[tondx] = b
			data[frmndx] = a
		
			mi = mi - 1
		return bytes(data)
			
	'''
		@sdescription:		This will encrypt the data using the specified
		@+:					key during creation of the SymCrypt class.
	'''
	def crypt(self, data):
		if self.so_crypt is not None:
			self.so_crypt(c_char_p(self.xkey), c_int(len(self.xkey)), c_char_p(self.mkey), c_int(len(self.mkey)), c_char_p(data), c_int(len(data)))
			return data
		return self.mix(self.__both(data))
	'''
		@sdescription:		This will decrypt the data using the specified
		@+:					key during creation of the SymCrypt class.
	'''
	def decrypt(self, data):
		if self.so_decrypt is not None:
			self.so_decrypt(c_char_p(self.xkey), c_int(len(self.xkey)), c_char_p(self.mkey), c_int(len(self.mkey)), c_char_p(data), c_int(len(data)))
			return data
		return self.__both(self.unmix(data))
		
class IDGen:
	def __init__(self, size):
		self.size = size
		self.gened = {}
	'''
		Generates a unique ID (could have been used before)
	'''
	def gen(size):
		o = []
		x = 0
		while x < size:
			o.append(random.randint(0, 255))
			x = x + 1
		return bytes(o)
	# TODO: add method to remove uid from self.gened once link has been dropped
	def urem(self, uid):
		if uid in self.gened:
			del self.gened[uid]
	'''
		Generates a unique (not used before) ID
	'''
	def ugen(self):
		while True:
			uid = IDGen.gen(self.size)
			if uid not in self.gened:
				self.gened[uid] = True
				return uid



