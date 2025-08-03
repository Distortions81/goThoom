/*
**	Network_dts.cp		dtslib2
**
**	Copyright 2023 Delta Tao Software, Inc.
** 
**	Licensed under the Apache License, Version 2.0 (the "License");
**	you may not use this file except in compliance with the License.
**	You may obtain a copy of the License at
** 
**		https://www.apache.org/licenses/LICENSE-2.0
** 
**	Unless required by applicable law or agreed to in writing, software
**	distributed under the License is distributed on an "AS IS" BASIS,
**	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
**	See the License for the specific language governing permissions and
**	imitations under the License.
*/

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "LinkedList_dts.h"
#include "Memory_dts.h"
#include "Network_dts.h"
#include "Utilities_dts.h"
#include "Network_cmn.h"


// this controls the temporary bandaid fix for a OSX 10.9+ issue
#define NEED_TCP_REOPEN		1


/*
**	Entry Routines
**
**	class DTSNetAddress
**	class DTSNetChannelPriv;
*/


/*
**	Protocol Documentation:
**
**	for this example:
**	the server's well-known TCP and UDP port numbers are both 5010
**	arbitrary/ephemeral addresses are 8000+
**
**	server opens tcp5010 and udp5010
**	server opens tcp8000
**
**	client opens tcp8000 and udp8000
**	client connects to server tcp5010
**
**	server accepts on tcp8000
**	server generates a unique identifier for this client
**	server sends identifier to client on tcp8000
**
**	client sends a special flag followed by the identifier back to the server at udp5010
**	client retries twice more if necessary
**
**	server stores the client udp address
**	server sends on tcp8000 a message informing the client we are now fully connected
**
**	client and server can now exchange messages through a paranoid firewall
**
**	server sends udp packets to all clients through udp5010
**	all clients send udp packets to the server at udp5010
**	when the server receives a packet on udp5010 it hashes up the from address to help
**	find the proper channel
**	that packet is added to the channel's queue
*/


/*
**	Definitions
*/
const ulong	kHandshakeTimeout	= 10 * 60;		// 10 seconds
const uint	kHandshakeTag		= 0x0FFFF;


// values of DTSConnectList::clState
enum ConnectState
{
	kConnectNotInited,
#if 0		// not needed for sockets
	kConnectReadyToListen,
	kConnectReadyToAccept,
#endif
	kConnectReadyToSendTCP,
	kConnectWaitForUDP,
	kConnectSendConfirm,
	kConnectConnected
};


/*
**	class DTSConnectList
**
**	linked list of client connections, at varying stages of progress
*/
class DTSConnectList : public DTSLinkedList<DTSConnectList>
{
public:
	DTSNetChannel *		clChannel;
//	DTSNetCall *		clCall;
	ulong				clUDPTimeOut;
	int32_t				clUniqueID;
	ConnectState		clState;
	
	// constructor/destructor
					DTSConnectList();
	virtual			~DTSConnectList()
						{
						delete clChannel;
//						DTSNetFree( clCall );
						}
	
private:
	// declared but not defined
					DTSConnectList( const DTSConnectList& );
					DTSConnectList& operator=( const DTSConnectList& );
};


/*
**	struct DTSQueue
**	list of unread UDP packets which the server hasn't
**	had a chance to process yet.
*/
struct DTSQueue : public DTSLinkedList<DTSQueue>
{
	size_t		qSize;
	char		qData[0];
};


/*
**	HostHash
**	hash table entry for mapping IP addresses to net channels
*/
struct DTSHostHash
{
	DTSNetChannel *		hhChannel;
	DTSNetAddress		hhAddress;
	
				DTSHostHash() :
					hhChannel( nullptr ),
					hhAddress()
				{
				}
				
				// we don't own the channel, so don't need to delete it
//				~DTSHostHash() {}
};


/*
**	Partial Packets
**	These semantics are applicable only to TCP (streamed) connections
*/
class PartialPacket
{
public:
	uchar *		ppBuffer;		// pointer to the buffer to hold the packet
	size_t		ppMaxSize;		// the size of the packet buffer
	size_t		ppSize;			// the size of the packet to be read
	size_t		ppRead;			// the number of bytes already read
	
	// constructor/destructor
				PartialPacket() :
						ppBuffer(),
						ppMaxSize(),
						ppSize(),
						ppRead()
					{
					}
	
				~PartialPacket()
					{
					delete[] ppBuffer;
					}
	
	// interface
	DTSError	Allocate();
	DTSError	ReadData( DTSEndPoint ep, void * buffer, size_t * sz );
	DTSError	FillPartialBuffer( DTSEndPoint ep );
	void		Reset()
					{
					ppSize = 0;
					ppRead = 0;
					}
	
private:
				// declared but not defined
				PartialPacket( const PartialPacket& );
				PartialPacket& operator=( const PartialPacket& );
};


/*
**	"partial" packets
**	alternate version, for UDP (datagram) connections
*/
class PartialPacketUDP
{
public:
	DTSError	ReadData( DTSEndPoint ep, DTSNetAddress& addr, void * buf, size_t * sz );
};


/*
**	DTSNetChannelPriv
*/
class DTSNetChannelPriv
{
public:
	DTSConnectList *	netConnectList;
	DTSNetChannelPriv *	netHostChannel;
	DTSQueue *			netQueue;			// actually a LIFO not a queue
	DTSHostHash *		netHostHash;		// array of hash entries
	DTSNetAddress		netDstAddr;
	PartialPacket		netTCPPacket;
	PartialPacketUDP	netUDPPacket;
	DTSEndPoint			netTCP;
	DTSEndPoint			netUDP;
	int					netHostMax;			// size (log2) of hash table
	bool				netConnected;
	ushort				netPort;
	
	// constructor/destructor
				DTSNetChannelPriv() :
					netConnectList(),
					netHostChannel(),
					netQueue(),
					netHostHash(),
					netDstAddr(),
					netTCPPacket(),
					netUDPPacket(),
					netTCP( kClosedEndPoint ),
					netUDP( kClosedEndPoint ),
					netHostMax(),
					netConnected( false ),
					netPort( 0 )
					{
					}
	
				~DTSNetChannelPriv()
					{
					Close();
					}
	
private:
	// declared but not defined
				DTSNetChannelPriv( const DTSNetChannelPriv& );
	DTSNetChannelPriv&
				operator=( const DTSNetChannelPriv& );
	
private:
	// private interface
	void		Close();
	DTSConnectList * FindConnection( ConnectState state ) const;
	DTSError	HandleConnectList();
	DTSError	CreateConnect( DTSEndPoint ep );
	bool		Handle1Connect( DTSConnectList * connect );
#if 0  // not needed for sockets
	bool		DoListenTask( DTSConnectList * connect );
	bool		DoAcceptTask( DTSConnectList * connect );
#endif
	bool		DoSendTCPTask( DTSConnectList * connect );
	bool		DoWaitUDPTask( DTSConnectList * connect );
	bool		DoConfirmTask( DTSConnectList * connect );
	DTSError	HandleAccept();
	
	DTSError	ConnectTCP( DTSNetChannel * pub );
	DTSError	ConnectUDP( DTSNetChannel * pub );
	DTSError	ConnectTCPWait( DTSNetChannel * pub, uint32_t * id );
	DTSError	ConnectUDPExchange( DTSNetChannel * pub, uint32_t id );
	void		FreeQueued();
	DTSError	HandleHostQueue();
	DTSError	HandleUDP();
	DTSError	Handle1UDP();
	DTSError	Handle1NewUDP( DTSNetAddress& srcaddr, uchar * buff, size_t count );
	DTSError	Handle1ExistingUDP( DTSNetAddress& srcaddr, uchar * buf, size_t ct );
	DTSError	ReadNetQueue( void * data, size_t * sz );
	DTSError	AddToHash( DTSNetChannel * channel );
	void		RemoveFromHash();
	DTSError	FindInHash( const DTSNetAddress& addr, DTSNetChannel ** channel );
	uint		HashFunction() const;
	DTSError	GetPeerAddress( DTSNetAddress& oAddr ) const;
	
	friend class DTSNetChannel;
};


/*
**	Internal Routines
*/
static uint		PrivHashFunction( const DTSNetAddress& addr, int shift );
static DTSError	TCPStringToAddr( DTSEndPoint ep, const char * str, DTSNetAddress& oAdr );


/*
**	Internal Variables
*/
int32_t gUniqueID;


/*
**	class DTSNetAddress (sockets version)
*/
DTSNetAddress::DTSNetAddress()
{
	memset( &addr, 0, sizeof addr );
}


/*
**	DTSNetAddress::DTSNetAddress()
**	constructor from (struct sockaddr)
*/
DTSNetAddress::DTSNetAddress( const sockaddr * inAddr )
{
	addr = * reinterpret_cast<const sockaddr_in *>( inAddr );
}


/*
**	DTSNetAddress::Set()
**	set to a different (struct sockaddr)
*/
void
DTSNetAddress::Set( const sockaddr * src )
{
	addr = * reinterpret_cast<const sockaddr_in *>( src );
}


/*
**	DTSNetAddress::Set()
**	init from family, address, port
*/
void
DTSNetAddress::Set( sa_family_t fam, in_addr_t address, in_port_t port /* = 0 */ )
{
	Reset();
	
#ifndef __linux
	// Linux doesn't have this field.
	// not sure if we need it for Darwin, either.
//	addr.sin_len = sizeof addr;
#endif
	addr.sin_family = fam;
	addr.sin_addr.s_addr = htonl( address );
	addr.sin_port = htons( port );
}


/*
**	DTSNetAddress::Reset()
**	zero out the address
*/
void
DTSNetAddress::Reset()
{
	memset( &addr, 0, sizeof addr );
}


/*
**	DTSNetAddress::operator==()
**	equality test
*/
bool
DTSNetAddress::operator==( const DTSNetAddress& other ) const
{
	// only family, IPv4, and port must match
	// (this doesn't account for wildcard addresses, etc.)
	return other.addr.sin_family		== addr.sin_family
		&& other.addr.sin_addr.s_addr	== addr.sin_addr.s_addr
		&& other.addr.sin_port   		== addr.sin_port;
}


/*
**	DTSNetChannelPriv
**
**	implement this class as quickly and as quietly as possible.
**	all routines call through to the private ones.
*/
DTSDefineImplementFirmNoCopy( DTSNetChannelPriv )


// these next two definitions cannot be visible until _after_ the
// DefineImplement() macro above, otherwise you get a 
// "specialization after instantiation" compiler error.
DTSNetChannel::DTSNetChannel() :
	priv()
{
}


DTSNetChannel::~DTSNetChannel()
{
}


/*
**	DTSNetChannel::Close()
*/
void
DTSNetChannel::Close()
{
	if ( DTSNetChannelPriv * p = priv.p )
		p->Close();
}


/*
**	DTSNetChannelPriv::Close()
**
**	close down the connection
**	and reset it
*/
void
DTSNetChannelPriv::Close()
{
	RemoveFromHash();
	DTSNetClose( netTCP );
	DTSNetClose( netUDP );
	FreeQueued();
	DTSConnectList::DeleteLinkedList( netConnectList );
	delete[] netHostHash;
	
	netTCP         = kClosedEndPoint;
	netUDP         = kClosedEndPoint;
	netConnected   = false;
	netHostChannel = nullptr;
//	netQueue       = nullptr;	// FreeQueued() and DeleteLL() ...
//	netConnectList = nullptr;	// ... do these already
	netHostHash    = nullptr;
	netHostMax     = 0;
	netPort        = 0;
	netDstAddr.Reset();
}


/*
**	DTSNetChannel::IsConnected()
**
**	return true if connected
*/
bool
DTSNetChannel::IsConnected() const
{
	const DTSNetChannelPriv * p = priv.p;
	return p ? p->netConnected : false;
}

#pragma mark --- Server Bits ---


/*
**	DTSNetChannel::InitHost()
**
**	[server]
**	initialize a well-known TCP port for listening
**	Prepare the hash table to track up to 'max' connections.
*/
DTSError
DTSNetChannel::InitHost( ushort port, uint max )
{
	DTSNetChannelPriv * p = priv.p;
	if ( not p )
		return -1;
	
	DTSEndPoint tcpep = kClosedEndPoint;
	DTSEndPoint udpep = kClosedEndPoint;
	
	DTSError result;
//	if ( noErr == result )
		result = DTSNetOpenTCP( &tcpep, &port, true );
	
	if ( noErr == result )
		result = DTSNetOpenUDP( &udpep, &port, true );
	
	// create the hash table, containing 'max' entries (rounded up to
	// the next higher power of 2). Set 'count' to log2() of that power.
	DTSHostHash * hash = nullptr;
	int count = 1;
	
	if ( noErr == result )
		{
		while ( max != 0 )
			{
			++count;
			max >>= 1;
			}
		hash = NEW_TAG("DTSNetHash") DTSHostHash[ 1 << count ];
		if ( not hash )
			result = memFullErr;
		}
	if ( noErr == result )
		{
		p->netTCP      = tcpep;
		p->netUDP      = udpep;
		p->netPort     = port;
		p->netHostHash = hash;
		p->netHostMax  = count;
		}
	
	// clean up after failures
	if ( noErr != result )
		{
		DTSNetClose( tcpep );
		DTSNetClose( udpep );
		delete[] hash;
		}
	
	return result;
}


/*
**	DTSNetChannel::AnswerClient()
**
**	[server]
**	service all established UDP connections, queuing data onto each channel
**	progress all pending connections
**	listen for new clients trying to connect
**	if a pending connection has completed the full handshake,
**	move it out of the pending pool into the hash table, and return it
*/
DTSError
DTSNetChannel::AnswerClient( DTSNetChannel ** pchannel )
{
	DTSNetChannelPriv * p = priv.p;
	if ( not p )
		return -1;
	
	// read & enqueue all inbound UDP packets
	DTSError result = p->HandleUDP();
	
	// advance all pending incomplete connections
	if ( noErr == result )
		result = p->HandleConnectList();
	
	// check for new connection requests
	if ( noErr == result )
		result = p->HandleAccept();
	
	// have any connections completed their handshakes?
	DTSNetChannel * channel = nullptr;
	if ( noErr == result )
		{
		DTSConnectList * connect = p->FindConnection( kConnectConnected );
		if ( connect )
			{
			// we have a winner!  Record it in the hash
			DTSNetChannel * test = connect->clChannel;
			test->priv.p->netHostChannel = p;
			result = p->AddToHash( test );
			
			// remove it from the pending list, and return it to our caller
			if ( noErr == result )
				{
				channel            = test;
				connect->clChannel = nullptr;
				connect->Remove( p->netConnectList );
				delete connect;
				}
			}
		}
	
	*pchannel = channel;
	return result;
}


/*
**	DTSNetChannelPriv::FindConnection()
**
**	[server]
**	find a connection in the given state
*/
DTSConnectList *
DTSNetChannelPriv::FindConnection( ConnectState state ) const
{
	for ( DTSConnectList * conn = netConnectList;  conn;  conn = conn->linkNext )
		{
		if ( conn->clState == state )
			return conn;
		}
	
	return nullptr;
}


/*
**	DTSNetChannelPriv::HandleConnectList()
**
**	[server]
**	progress all of the pending connections
*/
DTSError
DTSNetChannelPriv::HandleConnectList()
{
	bool bMoreToDo;
	do
		{
		bMoreToDo = false;
		
#if 0  // not needed for sockets
		// make sure someone is listening
		DTSConnectList * connect = FindConnection( kConnectReadyToListen );
		if ( not connect )
			{
			DTSError result = CreateConnect( kClosedEndPoint );
			if ( result != noErr )
				{
				// failure here would be really bad!
				return result;
				}
			}
#endif
		
		// do the periodic task for every pending connection
		DTSConnectList * next;
		for ( DTSConnectList * connect = netConnectList;  connect;  connect = next )
			{
			next = connect->linkNext;
			bool bDoMore = Handle1Connect( connect );
			if ( bDoMore )
				bMoreToDo = true;
			}
		}
	while ( bMoreToDo );
	
	return noErr;
}


/*
**	DTSNetChannelPriv::HandleAccept()
**
**	[server]
**	accept new TCP connections and add them to the connectList
*/
DTSError
DTSNetChannelPriv::HandleAccept()
{
	// accept an inbound TCP connection request, if there is one
	DTSEndPoint newEP = kClosedEndPoint;
	DTSError result = DTSNetAccept( netTCP, &newEP, nullptr );
	if ( noErr == result )
		{
		// got one. Create a new ConnectList to track its handshake.
		result = CreateConnect( newEP );
		
		// Hmm: accept() provides the caller's address; maybe we should grab it now
		// and then verify it later, in Handle1NewUDP?  DTSNetAccept() would need
		// a touchup...
		return result;
		}
	
	// if nobody's calling, that's OK too
	if ( result > 0 )
		return noErr;
	
	// we shouldn't get here unless the accept() failed
	
#if defined( DTS_XCODE ) && NEED_TCP_REOPEN
	// ad-hoc fix for OSX 10.9+ viciously closing our TCP listening endpoint
	// out from underneath us. I'm not sure that I fully understand the issue;
	// this just masks the disease but probably doesn't cure it.
	
	// close the old listening endpoint, open a new one
	DTSNetClose( netTCP );
	ushort port = netPort;
	result = DTSNetOpenTCP( &netTCP, &port, true );
#endif  // DTS_XCODE
	
	return result;
}


/*
**	DTSNetChannelPriv::CreateConnect()
**
**	[server]
**	create a new connection ready to listen
*/
DTSError
DTSNetChannelPriv::CreateConnect( DTSEndPoint tcpep )
{
	DTSNetChannel * channel = nullptr;
//	DTSEndPoint tcpep = kClosedEndPoint;
//	DTSNetCall * call = nullptr;
	
	DTSError result = noErr;
	DTSConnectList * connect = NEW_TAG("DTSConnectList") DTSConnectList;
	if ( not connect )
		result = memFullErr;
	
	if ( noErr == result )
		{
		channel = NEW_TAG("DTSNetChannel") DTSNetChannel;
		if ( not channel )
			result = memFullErr;
		else
		if ( not channel->priv.p )
			{
			delete channel;
			channel = nullptr;
			result = memFullErr;
			}
		}
#if (defined( DTS_Mac ) && defined( NETWORK_MAC_H ))	\
 || (defined( DTS_USE_XTI ) && DTS_USE_XTI)
	// don't do this for Sockets; only XTI and OpenTpt.
	// however, in this file, we don't know which model is in use, so we have to 
	// test DTS_Mac/DTS_USE_XTI instead.
	
	if ( noErr == result )
		result = DTSNetOpenTCP( &tcpep, nullptr, false );
	
	if ( noErr == result )
		result = DTSNetAlloc( tcpep, nullptr /*&call*/ );
#endif  // OpenTransport / XTI
	
	if ( noErr == result )
		{
		channel->priv.p->netTCP = tcpep;
		connect->clState      = kConnectReadyToSendTCP; // kConnectReadyToListen;
		connect->clChannel    = channel;
//		connect->clCall       = call;
		
#if 1
		// avoid excessive create/delete churning of useless connects:
		// process all pending accepts _before_ the listen
		connect->InstallLast( netConnectList );
#else
		// this original order may be necessary for XTI
		connect->InstallFirst( netConnectList );
#endif
		}
	
	if ( noErr != result )
		{
		delete connect;
		delete channel;
		DTSNetClose( tcpep );
//		DTSNetFree( call );
		}
	
	return result;
}


/*
**	DTSNetChannelPriv::Handle1Connect()
**
**	[server]
**	progress one pending connection via state machine
*/
bool
DTSNetChannelPriv::Handle1Connect( DTSConnectList * connect )
{
	bool bMoreToDo = false;
	
	ConnectState state = connect->clState;
	switch ( state )
		{
#if 0  // not needed for sockets
		// ready to listen
		case kConnectReadyToListen:
			bMoreToDo = DoListenTask( connect );
			break;
		
		// ready to accept
		case kConnectReadyToAccept:
			bMoreToDo = DoAcceptTask( connect );
			break;
#endif  // 0
		
		// ready to send the unique ID over TCP
		case kConnectReadyToSendTCP:
			bMoreToDo = DoSendTCPTask( connect );
			break;
		
		// wait for the UDP to connect
		case kConnectWaitForUDP:
			bMoreToDo = DoWaitUDPTask( connect );
			break;
		
		// send the confirmation on TCP
		case kConnectSendConfirm:
			bMoreToDo = DoConfirmTask( connect );
			break;
		
		// already connected waiting for the caller
		case kConnectConnected:
			break;
		
		// must be an error
		default:
			connect->Remove( netConnectList );
			delete connect;
			break;
		}
	
	return bMoreToDo;
}


#if 0	// not needed for sockets
/*
**	DTSNetChannelPriv::DoListenTask()
**
**	[server]
**	listen for connections
*/
bool
DTSNetChannelPriv::DoListenTask( DTSConnectList * connect )
{
	// listen for incoming connection request
	// For XTI this used to call t_listen(); but for sockets it merely tests
	// the readability of the listening socket. Returns +1 if no one's knocking,
	// or 0 if there IS a caller, or a negative error code if anything went kablooey.
	DTSError result = DTSNetListen( netTCP, nullptr /*connect->clCall*/ );
	
	// no one is calling
	// no more to do
	if ( result > 0 )
		return false;
	
	// error, nuke em
	if ( result < 0 )
		{
		connect->Remove( netConnectList );
		delete connect;
		return false;
		}
	
	// success, change to the next state
	// and there is more to do
	connect->clState = kConnectReadyToAccept;
	return true;
}


/*
**	DTSNetChannelPriv::DoAcceptTask()
**
**	[server]
**	accept a connection
*/
bool
DTSNetChannelPriv::DoAcceptTask( DTSConnectList * connect )
{
	// accept
	DTSEndPoint hostep = netTCP;
	DTSNetChannel * channel = connect->clChannel;
	DTSError result = DTSNetAccept( hostep,
						&channel->priv.p->netTCP, nullptr /*connect->clCall*/ );
	
	// success, change to the next state
	// and there is more to do
	if ( noErr == result )
		{
		connect->clState = kConnectReadyToSendTCP;
		return true;
		}
	
	// listen event, stay in this state
	// and there's more to do
	// specifically, someone needs to listen
	// and we need to accept again
	// (timmmer thinks this is a rather stupid interface)
	// [can only happen with XTI]
	if ( kNetworkListen == result )
		return true;
	
	// we shouldn't get here unless the accept() failed
	
#if defined( DTS_XCODE ) && NEED_TCP_REOPEN
	// ad-hoc fix for OSX 10.9+ viciously closing our TCP listening
	// endpoint out from underneath us
	if ( kNetworkNoData == result )
		{
		// close the old listening endpoint, open a new one
		DTSNetClose( netTCP );
		ushort port = netPort;
		result = DTSNetOpenTCP( &netTCP, &port, true );
		}
#endif  // DTS_XCODE
	
	// some other error
	// give up
	connect->Remove( netConnectList );
	delete connect;
	return false;
}
#endif  // XTI only


/*
**	DTSNetChannelPriv::DoSendTCPTask()
**
**	[server]
**	send the client their unique identifier over TCP
*/
bool
DTSNetChannelPriv::DoSendTCPTask( DTSConnectList * connect )
{
	// send the unique identifier to the client on the reliable TCP endpoint
	DTSNetChannel * channel = connect->clChannel;
	int32_t uid = NativeToBigEndian( connect->clUniqueID );
	DTSError result = DTSNetSend( channel->priv.p->netTCP, &uid, sizeof uid );
	
	// error, nuke em
	if ( noErr != result )
		{
		connect->Remove( netConnectList );
		delete connect;
		return false;
		}
	
	// success!
	// go to the next state
	connect->clState      = kConnectWaitForUDP;
	connect->clUDPTimeOut = GetFrameCounter() + 3 * kHandshakeTimeout;
	
	// we're done for now
	return false;
}


/*
**	DTSNetChannelPriv::DoWaitUDPTask()
**
**	[server]
**	wait for the client to send their unique identifier back via UDP
*/
bool
DTSNetChannelPriv::DoWaitUDPTask( DTSConnectList * connect )
{
	// we only handle the timeout here;
	// the change of state happens in Handle1NewUDP() when
	// the unique identifier arrives
	ulong timeout = connect->clUDPTimeOut;
	ulong curTime = GetFrameCounter();
	if ( curTime > timeout )
		{
		// timed out
		// nuke em
		connect->Remove( netConnectList );
		delete connect;
		}
	
	// we never have more to do
	return false;
}


/*
**	DTSNetChannelPriv::DoConfirmTask()
**
**	[server]
**	complete the client-server connection handshake
*/
bool
DTSNetChannelPriv::DoConfirmTask( DTSConnectList * connect )
{
	// send the confirmation data
	// the value here is irrelevant, as long as it's two bytes
	const int16_t buff = 0x7FFF;
	DTSNetChannel * channel = connect->clChannel;
	DTSError result = DTSNetSend( channel->priv.p->netTCP, &buff, sizeof buff );
	
	// we are now fully connected
	if ( noErr == result )
		{
#if 1
		// mark the channel as being connected
		// (to keep GetPeerAddress() happy)
		// Not sure if this is the best place to set the flag, though...
		// Maybe it should happen at the end of AnswerClient() instead.
		if ( channel && channel->priv.p )
			channel->priv.p->netConnected = true;
#endif	// 1
		
		connect->clState = kConnectConnected;
		return false;
		}
	
	// check for timeout
	ulong timeout = connect->clUDPTimeOut;
	ulong curTime = GetFrameCounter();
	if ( curTime > timeout )
		result = kNetworkHandshakeTimeout;
	
	// error, nuke em
	if ( noErr > result )
		{
		connect->Remove( netConnectList );
		delete connect;
		}
	
	// we're done
	return false;
}


/*
**	DTSConnectList::DTSConnectList()
**
**	[server]
**	constructor
*/
DTSConnectList::DTSConnectList() :
	clChannel(),
//	clCall(),
	clUDPTimeOut(),
	clUniqueID( ++gUniqueID ),
	clState( kConnectNotInited )
{
#if 0	// no longer an issue
	// update the unique identifier
	// in order to completely avoid accidentally establishing
	// a connection with an older and incompatible version of this library...
	// which used to send a two byte port number...
	// we make sure the top two bytes of this unique id are not in the range
	// used by any application we have shipped so far.
	// clan lord used ports near and above 5000
	// skip everything up to 5099
	// just to be paranoidally safe.
	uint id = gUniqueID + 1;
	uint oldport = ( id >> 16 ) & 0x0000FFFF;
	if ( oldport >= 5000
	&&   oldport <= 5099 )
		{
		id = (5100U << 16) + (id & 0x0000FFFF);
		}
	gUniqueID  = id;
	clUniqueID = id;
#endif  // 0
}


#pragma mark --- Client Bits ---

/*
**	DTSNetChannel::ConnectHost()
**
**	[client]
**	connect to the host at the given address
**	'portin' is the port number to use on the -local- side;
**	pass 0 to request an ephemeral (unused, high-numbered) one.
*/
DTSError
DTSNetChannel::ConnectHost( const char * hoststring, ushort portin )
{
	DTSNetChannelPriv * p = priv.p;
	if ( not p )
		return -1;
	
	// handle if the client requested a specific port
	ushort * pport = nullptr;
	if ( portin )
		pport = &portin;
	
	DTSError result = DTSNetOpenTCP( &p->netTCP, pport, false );
	if ( noErr == result )
		result = DTSNetOpenUDP( &p->netUDP, pport, false );
	
	// obtain server's IP address
	if ( noErr == result )
		result = TCPStringToAddr( p->netTCP, hoststring, p->netDstAddr );
	
	// attempt the hookup
	if ( noErr == result )
		result = p->ConnectTCP( this );
	if ( noErr == result )
		result = p->ConnectUDP( this );
	
	// are we online?
	if ( noErr == result )
		p->netConnected = true;
	
	return result;
}


/*
**	DTSNetChannelPriv::ConnectTCP()
**
**	[client]
**	establish the TCP connection
*/
DTSError
DTSNetChannelPriv::ConnectTCP( DTSNetChannel * pub )
{
	DTSEndPoint ep = netTCP;
	DTSError result = pub->ConnectCallBack( "Connecting to host." );
	if ( noErr == result )
		result = DTSNetConnect( ep, netDstAddr, pub );
	if ( noErr == result )
		result = pub->ConnectCallBack( "Established reliable connection." );
	
	return result;
}


/*
**	DTSNetChannelPriv::ConnectUDP()
**
**	[client]
**	establish the UDP connection
*/
DTSError
DTSNetChannelPriv::ConnectUDP( DTSNetChannel * pub )
{
	uint32_t id = 0;
	
	// wait for the unique identifier to arrive on the TCP endpoint
	DTSError result = pub->ConnectCallBack(
						"Waiting for unreliable connection from host." );
	if ( noErr == result )
		result = ConnectTCPWait( pub, &id );
	
	// great; now we have the unique identifier
	// send it back to the server on the UDP port
	// wait for confirmation on the TCP port
	if ( noErr == result )
		result = pub->ConnectCallBack( "Establishing unreliable connection." );
	if ( noErr == result )
		result = ConnectUDPExchange( pub, id );
	
	// first retry
	if ( kNetworkHandshakeTimeout == result )
		{
		result = pub->ConnectCallBack( "First unreliable connection retry." );
		if ( noErr == result )
			result = ConnectUDPExchange( pub, id );
		}
	// second and last retry
	if ( kNetworkHandshakeTimeout == result )
		{
		result = pub->ConnectCallBack( "Second and last unreliable connection retry." );
		if ( noErr == result )
			result = ConnectUDPExchange( pub, id );
		}
	if ( noErr == result )
		result = pub->ConnectCallBack( "Unreliable connection established." );
	
	return result;
}


/*
**	DTSNetChannelPriv::ConnectTCPWait()
**
**	[client]
**	wait for the unique identifier to arrive on the TCP endpoint
*/
DTSError
DTSNetChannelPriv::ConnectTCPWait( DTSNetChannel * pub, uint32_t * id )
{
	ulong stopTime = GetFrameCounter() + kHandshakeTimeout;
	DTSError result;
	DTSEndPoint tcpep = netTCP;
	for (;;)
		{
		// receive our unique 32-bit identifier (in network byte order)
		size_t sz = sizeof *id;
		result = DTSNetReceive( tcpep, id, &sz );
		
		// bail on success
		// bail on hard error
		if ( noErr >= result )
			break;
		
		// check for timeout
		ulong curTime = GetFrameCounter();
		if ( curTime > stopTime )
			{
			result = kNetworkHandshakeTimeout;
			break;
			}
		
		// call the callback routine
		result = pub->ConnectCallBack( nullptr );
		if ( noErr != result )
			break;
		}
	
	return result;
}


/*
**	DTSNetChannelPriv::ConnectUDPExchange()
**
**	[client]
**	send the unique id back to the server on the UDP port
**	wait for confirmation on the TCP port
*/
DTSError
DTSNetChannelPriv::ConnectUDPExchange( DTSNetChannel * pub, uint32_t id )
{
	DTSError result;
	// if ( noErr == result )
		{
		// since the listener (server) is expecting complete packets to arrive
		// on the UDP port, we had better send it something it can
		// at least recognize as a legitimate packet.
		// we set the "size" to be 0xFFFF, aka kHandshakeTag,
		// and after that we send 4 bytes with the unique identifier.
		// the server doesn't know about us yet.
		// and it ignores packets from unknown senders
		// unless they are in this format.
		// in which case it will remember us.
#if ! SEND_MULTI
		uchar buff[ 6 ];
		buff[0] = (uchar)( kHandshakeTag >> 8);
		buff[1] = (uchar)( kHandshakeTag >> 0);
		buff[2] = id >> 24;
		buff[3] = id >> 16;
		buff[4] = id >>  8;
		buff[5] = id;
		
		result = DTSNetUSend( netUDP, netDstAddr, buff, sizeof buff );
#else
		// no need to byte-swap 'id' here on little-endian, because it
		// was never de-swapped after ConnectTCPWait().
		const uint16_t tag = kHandshakeTag;
		result = DTSNetUSendMulti( netUDP, netDstAddr,
					&tag, sizeof tag, &id, sizeof id );
#endif	// SEND_MULTI
		}
	
	// wait for the confirmation to arrive on the TCP channel
	if ( noErr == result )
		{
		ulong stopTime = GetFrameCounter() + kHandshakeTimeout;
		DTSEndPoint tcpep = netTCP;
		for (;;)
			{
			// receive the confirmation
			int16_t dummy;
			size_t sz = sizeof dummy;
			result = DTSNetReceive( tcpep, &dummy, &sz );
			
			// bail on success
			// bail on hard error
			if ( noErr >= result )
				break;
			
			// check for timeout
			ulong curTime = GetFrameCounter();
			if ( curTime > stopTime )
				{
				result = kNetworkHandshakeTimeout;
				break;
				}
			
			// call the call back routine
			result = pub->ConnectCallBack( nullptr );
			if ( noErr != result )
				break;
			}
		}
	
	return result;
}


/*
**	DTSNetChannel::ConnectCallBack()
**
**	[client]
**	the default callback routine does nothing.
**	subclasses can override this.
*/
DTSError
DTSNetChannel::ConnectCallBack( const char * /* status */ )
{
	return noErr;
}


#pragma mark --- Common Bits ---

/*
**	DTSNetChannel::Write()
**
**	[client/server]
**	write data, over the indicated channel (TCP or UDP)
*/
DTSError
DTSNetChannel::Write( const void * data, size_t sz, int packetType )
{
	DTSNetChannelPriv * p = priv.p;
	if ( not p )
		return -1;
	
	// better not try to send more than 64K - 2 bytes,
	// nor a value that could be misconstrued as the handshake tag.
	if ( sz >= kHandshakeTag - sizeof(short) )
		sz = kHandshakeTag - sizeof(short) - 1;
	
	// clear UDP packets from the host channel
	// (a no-op in the client, since netHostChannel is always nil there)
	p->HandleHostQueue();
	
	DTSError result = noErr;
	
#if ! SEND_MULTI
	size_t msgsize = sz + sizeof(short);
	
	// convert the raw data to a message (w/leading 2-byte size field)
	char * ptr = NEW_TAG("DTSNetWriteTemp") char[ msgsize ];
	if ( not ptr )
		result = memFullErr;
	
	// send the message reliably with TCP
	// or unreliably with UDP
	if ( noErr == result )
		{
		ptr[0] = sz >> 8;
		ptr[1] = sz & 0x0FF;
		memcpy( ptr + 2, data, sz );
		
		if ( kNetWriteReliable == packetType )
			result = DTSNetSend( p->netTCP, ptr, msgsize );
		else
			{
			DTSEndPoint ep = p->netUDP;
			if ( not IsEndPointOpen(ep) )
				{
				if ( DTSNetChannelPriv * host = p->netHostChannel )
					ep = host->netUDP;
				}
			if ( IsEndPointOpen(ep) )
				result = DTSNetUSend( ep, p->netDstAddr, ptr, msgsize );
			else
				result = -1;
			}
		}
	delete[] ptr;
	
#else	// SEND_MULTI
	
	// two-buffer version via writev() et al. -- simpler code, no new/delete/memcpy
	uint16_t msgsize = NativeToBigEndian( static_cast<ushort>( sz )	);
	
	if ( kNetWriteReliable == packetType )
		result = DTSNetSendMulti( p->netTCP, &msgsize, sizeof msgsize, data, sz );
	else
		{
		DTSEndPoint ep = p->netUDP;
		if ( not IsEndPointOpen( ep ) )
			{
			if ( DTSNetChannelPriv * host = p->netHostChannel )
				ep = host->netUDP;
			}
		if ( IsEndPointOpen( ep ) )
			result = DTSNetUSendMulti( ep, p->netDstAddr, &msgsize, sizeof msgsize,
						data, sz );
		else
			result = -1;
		}
#endif	// SEND_MULTI
	
	// track disconnects
	if ( kNetworkDisconnect == result )
		p->netConnected = false;
	
	return result;
}


/*
**	DTSNetChannel::Read()
**
**	[client/server]
**	read data, from both TCP and UDP
**	(99% of the time, there'll be nothing on TCP)
*/
DTSError
DTSNetChannel::Read( void * data, size_t * psize )
{
	DTSNetChannelPriv * p = priv.p;
	if ( not p )
		return -1;
	
	// clear UDP packets from the host channel (no-op on client)
	p->HandleHostQueue();
	
	// read from the reliable TCP endpoint
	size_t maxtoread = *psize;
	DTSError result = p->netTCPPacket.ReadData( p->netTCP, data, psize );
	
	// if nothing there, try to read unreliable data instead
	if ( noErr < result )
		{
		// read from a client's unreliable UDP endpoint
		DTSEndPoint ep = p->netUDP;
		*psize = maxtoread;
		if ( IsEndPointOpen(ep) )
			result = p->netUDPPacket.ReadData( ep, p->netDstAddr, data, psize );
		// or read from a host's queue (also a no-op on client)
		else
			result = p->ReadNetQueue( data, psize );
		}
	
	// track disconnects
	if ( kNetworkDisconnect == result )
		p->netConnected = false;
	
	return result;
}


/*
**	DTSNetChannel::GetPeerAddress()
**
**	[client/server]
**	returns address of remote peer, if we know it
*/
DTSError
DTSNetChannel::GetPeerAddress( DTSNetAddress& oAddr ) const
{
	DTSNetChannelPriv * p = priv.p;
	return p ? p->GetPeerAddress( oAddr ) : -1;
}


/*
**	PartialPacket::Allocate()
**
**	allocate memory for the packet
*/
DTSError
PartialPacket::Allocate()
{
	// replace the buffer if it is too small
	DTSError result = noErr;
	size_t maxSz   = ppMaxSize;
	size_t sz	   = ppSize;
	
	// check the size of the buffer against the size needed
	if ( maxSz < sz )
		{
		// allocate a new buffer
		uchar * newbuff = NEW_TAG("DTSNetPacket") uchar[ sz ];
		
		// bail if failed
		if ( not newbuff )
			result = memFullErr;
		
		// delete the old buffer
		// save the new one and its size
		else
			{
			delete[] ppBuffer;
			ppBuffer  = newbuff;
			ppMaxSize = sz;
			}
		}
	
	return result;
}


/*
**	PartialPacket::ReadData()
**
**	return complete data from the packet and reset
*/
DTSError
PartialPacket::ReadData( DTSEndPoint ep, void * data, size_t * sz )
{
	size_t numread = 0;
	
	// check the partial buffer
	DTSError result = FillPartialBuffer( ep );
	
	// copy the data from the partial buffer
	if ( noErr == result )
		{
		// pin to the size of the destination
		numread = *sz;
		size_t numinbuff = ppSize;
		if ( numread >= numinbuff )
			numread = numinbuff;
		memcpy( data, ppBuffer, numread );
		
		// reset the partial packet
		Reset();
		}
	
	*sz = numread;
	return result;
}


/*
**	PartialPacket::FillPartialBuffer()
**
**	fill the partial buffer from TCP
*/
DTSError
PartialPacket::FillPartialBuffer( DTSEndPoint ep )
{
	DTSError result = noErr;
	size_t count;
	uint16_t buff;
	
	// are we starting a new packet?
	if ( 0 == ppSize )
		{
		// read the size of the packet
		// if ( noErr == result )
			{
			count = sizeof buff;
			result = DTSNetReceive( ep, &buff, &count );
			}
		// allocate memory for the packet
		if ( noErr == result )
			{
			// check the packet size for the special handshake value
			// and ignore it
			if ( kHandshakeTag == buff )	// wrong-endian, but -1 == -1
				result = 1;
			else
				{
				ppSize = BigToNativeEndian( buff );
				result = Allocate();
				}
			}
		}
	
	// attempt to read data
	if ( noErr == result )
		{
		// check if the packet is partially full
		size_t numRead   = ppRead;
		size_t numToRead = ppSize - numRead;
		if ( numToRead > 0 )
			{
			// if ( noErr == result )
				{
				count = numToRead;
				uchar * dest = ppBuffer + numRead;
				result = DTSNetReceive( ep, dest, &count );
				}
			if ( noErr == result )
				ppRead = numRead + count;
			}
		}
	
	// change the error code if the packet is not yet full
	if ( noErr == result )
		{
		if ( ppSize > ppRead )
			result = kNetworkNoData;
		}
	
	return result;
}


/*
**	PartialPacketUDP::ReadData()
**
**	With datagrams, we know that either the whole 'gram will be available for reading
**	in a single gulp, or none at all. So we don't need the growable buffer.
**	What's more, legitimate clients will never send us more than (approximately)
**	(2 + sizeof(PlayerInput)) bytes per packet; and we're utterly uninterested in
**	anything larger that might come in from _illegitimate_ clients.
**	So we can read the entire packet into a local buffer all at once,
**	and dole it out to our caller.
**	This is easier, or at least more efficient, if we have RCV_MULTI:
**	that avoids at least one extra memcpy().
*/
DTSError
PartialPacketUDP::ReadData( DTSEndPoint ep, DTSNetAddress& addr,
	void * data, size_t * sz )
{
#if RCV_MULTI
	uint16_t frameSz = 0;
	size_t nRead = *sz, headerLen = sizeof frameSz;
	DTSError result = DTSNetURcvMulti( ep, addr, &frameSz, &headerLen, data, &nRead );
	if ( noErr == result )
		{
		if ( headerLen < sizeof frameSz )
			result = kNetworkNoData;
		else
			{
			frameSz = BigToNativeEndian( frameSz );
			if ( kHandshakeTag == frameSz )
				{
				nRead = 0;
				result = +1;
				}
			else
			if ( frameSz != nRead )
				result = kNetworkNoData;
			}
		}
#else  // ! RCV_MULTI
	uchar buff[ 2048 ];	// a nice round number > kMaxMsgSize
		// in principle this could even be as small as 534 (PlayerInput+2)
		// but let's be extravagantly lavish, eh?
	
	// read as much data as caller requests, but not to exceed sizeof(buff)
	size_t nRead = *sz;
	if ( nRead > sizeof buff )
		nRead = sizeof buff;
	
	// zero out the frame-length part, paranoically
	memset( buff, 0, sizeof(uint32_t) );
	
	DTSError result = DTSNetUReceive( ep, addr, buff, &nRead );
	if ( noErr == result )
		{
		// check for and discard the special handshake tag
		uint16_t tagOrLen = static_cast<uint16_t>( (buff[0] << 8) + buff[1] );
		if ( nRead >= 2
		&&   kHandshakeTag == tagOrLen )
			{
			nRead = 0;
			result = +1;
			}
		else
		if ( nRead == tagOrLen + sizeof tagOrLen )
			{
			// we got all the data we wanted:
			// copy the data; update the size
			memcpy( data, buff + sizeof tagOrLen, tagOrLen );
			nRead = tagOrLen;
			}
		else
			{
			// incomplete (or excessive?) read
			nRead = 0;
			result = kNetworkNoData;
			}
		}
#endif  // RCV_MULTI
	
	*sz = nRead;
	return result;
}


/*
**	TCPStringToAddr()
**
**	[client]
**	convert a string like "123.45.67.8:5000" or "server.clanlord.com:5010"
**	to the IP address the transport layer is looking for
*/
DTSError
TCPStringToAddr( DTSEndPoint ep, const char * str, DTSNetAddress& addr )
{
	// check for a string of format "123.45.67.8:5000"
	uint n1, n2, n3, n4;
	ushort port;
	int num = sscanf( str, "%u.%u.%u.%u:%hu", &n1, &n2, &n3, &n4, &port );
	if ( 5 == num )
		{
		// filter out garbage input
		if ( n1   > 255
		||   n2   > 255
		||   n3   > 255
		||   n4   > 255 )
			{
			return kNetworkBadTCPAddress;
			}
		
		// In fact this function really belongs in the per-platform code,
		// so it can use appropriate API's like inet_pton(), OTInetStringToHost(), etc.
		in_addr_t ip = (n1 << 24) | (n2 << 16) | (n3 << 8) | n4;
		addr.Set( AF_INET, ip, port & 0x0FFFF );
		
		return noErr;
		}
	
	// check for a domain name with a port number
	char hostname[ 256 ];
	const char * colon = strrchr( str, ':' );
	if ( colon > str )
		{
		port = ushort( atoi( colon + 1 ) );
		BufferToStringSafe( hostname, sizeof hostname,
				str, static_cast<size_t>( colon - str ) );
		str = hostname;
		}
	
	// resolve just the domain name
	// be paranoid
	DTSError result = DTSNetResolveName( ep, str, addr );
	
	// fold the port # back in, if any
	if ( noErr == result
	&&   colon != 0 )
		{
		addr.Set( addr.Family(), addr.IPAddr(), port );
		}
	
	return result;
}


/*
 **	DTSNetAddress::ToString()
**
**	This is essentially just a wrapper for inet_ntop(3).
**	'dst' points to a writable buffer of at least 'dstLen' bytes.
**	On successful return, 'dst' holds the dotted-decimal ASCII form of the address,
**	e.g. "127.0.0.1", otherwise it's empty.
**	Returns 'dst'.
*/
const char *
DTSNetAddress::ToString( char * dst, size_t dstLen ) const
{
#ifdef DEBUG_VERSION
	// paranoia
	if ( dstLen < INET_ADDRSTRLEN )
		{
		*dst = '\0';
		return dst;
		}
	// not that we ever expect to see IPv6 addresses, but just in case...
	if ( AF_INET6 == Family() && dstLen < INET6_ADDRSTRLEN )
		{
		*dst = '\0';
		return dst;
		}
#endif
	
	const char * str = inet_ntop( addr.sin_family, &addr.sin_addr.s_addr,
							dst, socklen_t( dstLen ) );
	if ( not str )
		{
		// can this even happen?
		*dst = '\0';
		}
	
	return dst;
}


#pragma mark --- More Server Bits ---

/*
**	DTSNetChannelPriv::HandleHostQueue()
**
**	[server]
**	read UDP packets from the host channel
*/
DTSError
DTSNetChannelPriv::HandleHostQueue()
{
	if ( DTSNetChannelPriv * host = netHostChannel )
		return host->HandleUDP();
	
	return noErr;
}


/*
**	DTSNetChannelPriv::HandleUDP()
**
**	[server]
**	read all UDP packets from the host channel
*/
DTSError
DTSNetChannelPriv::HandleUDP()
{
	DTSError result;
	for (;;)
		{
		result = Handle1UDP();
		if ( kNetworkNoData == result )
			{
			result = noErr;
			break;
			}
		if ( noErr > result )
			break;
		}
	
	return result;
}


/*
**	DTSNetChannelPriv::Handle1UDP()
**
**	[server]
**	read one UDP packet from the host channel.
**	Largely redone for sockets, because that truncates
**	unread portions of UDP datagrams, unlike XTI.
*/
DTSError
DTSNetChannelPriv::Handle1UDP()
{
	uchar buff[ 2048 ];			// kMaxMsgSize + a bit to spare
	size_t count = sizeof buff;
	DTSNetAddress srcaddr;
	
	// using DTSNetURcvMulti() doesn't profit us any here
	DTSError result = DTSNetUReceive( netUDP, srcaddr, buff, &count );
	if ( noErr == result )
		{
		if ( count < sizeof(int16_t) )
			{
			// we only read one byte for a two byte count?
			// we might be hosed.
			// but we might miraculously be okay
			// return a soft error
			result = +1;
			}
		}
	
	// is this a new or old UDP connection?
	if ( noErr == result )
		{
		size_t headerLen = static_cast<uint>( (buff[0] << 8) + buff[1] );
		if ( kHandshakeTag == headerLen )
			result = Handle1NewUDP( srcaddr,
						buff + sizeof(int16_t), count - sizeof(int16_t) );
		else
		// ensure the claimed payload size matches reality
		if ( headerLen == count - sizeof(int16_t) )
			result = Handle1ExistingUDP( srcaddr, buff + sizeof(int16_t), headerLen );
		else
			{
			// count is nonsensical?
			// we might be hosed.
			// but we might miraculously be okay
			// return a soft error
			result = +1;
			}
		}
	
	return result;
}


/*
**	DTSNetChannelPriv::Handle1NewUDP()
**
**	[server]
**	this packet is from a newly connecting client
*/
DTSError
DTSNetChannelPriv::Handle1NewUDP( DTSNetAddress& srcaddr, uchar * buff, size_t count )
{
	// we've already seen the 2-byte handshake tag, 0xFFFF.
	// now look for the 4-byte unique ID -- see ConnectUDPExchange()
	
	DTSError result = noErr;
//	if ( noErr == result )
		{
		if ( count < sizeof(int32_t) )
			{
			// um, we couldn't completely read the data
			// we might be hosed
			// but we might miraculously be okay
			// return a soft error
			result = +1;
			}
		}
	if ( noErr == result )
		{
		// convert to native byte order
		int id = ( buff[0] << 24 ) 
		       + ( buff[1] << 16 )
		       + ( buff[2] <<  8 )
		       + ( buff[3]       );
		
		// try to match it up amongst the pending connections
		for ( DTSConnectList * conn = netConnectList;  conn;  conn = conn->linkNext )
			{
			if ( kConnectWaitForUDP == conn->clState
			&&   conn->clUniqueID == id )
				{
				// bingo! save client's IP address, and advance to next conn state
				conn->clState = kConnectSendConfirm;
				DTSNetChannelPriv * p = conn->clChannel->priv.p;
				p->netDstAddr = srcaddr;
				break;
				}
			}
		}
	
	return result;
}


/*
**	DTSNetChannelPriv::Handle1ExistingUDP()
**
**	[server]
**	get the tail part of a UDP packet from the host channel
**	stuff it into the appropriate client channel's packet queue
*/
DTSError
DTSNetChannelPriv::Handle1ExistingUDP( DTSNetAddress& srcaddr,
	uchar * buff, size_t count )
{
	// encapsulate the data into a new DTSQueue
	DTSError result = noErr;
	DTSNetChannel * channel = nullptr;
	
	DTSQueue * queue = reinterpret_cast<DTSQueue *>(
							NEW_TAG("DTSNetQueue") char[ sizeof(DTSQueue) + count ] );
	if ( not queue )
		result = memFullErr;
	
	// look up client's channel in the IP-address hashtable
	if ( noErr == result )
		{
		// queue->linkNext = nullptr;
		memcpy( queue->qData, buff, count );
		result = FindInHash( srcaddr, &channel );
		}
	// hook it up
	if ( noErr == result && channel )
		{
		queue->qSize = count;
		queue->InstallLast( channel->priv.p->netQueue );
		}
	
	if ( noErr != result )
		delete[] reinterpret_cast<char *>( queue );
	
	return result;
}


/*
**	DTSNetChannelPriv::ReadNetQueue()
**
**	[server]
**	pop the first UDP packet off of the queue, and return its data
*/
DTSError
DTSNetChannelPriv::ReadNetQueue( void * data, size_t * psize )
{
	// bail if there is nothing in the queue
	DTSQueue * queue = netQueue;
	if ( not queue )
		{
		*psize = 0;
		return kNetworkNoData;
		}
	
	// copy the buffer
	size_t maxtocopy = *psize;
	size_t qsize     = queue->qSize;
	if ( maxtocopy > qsize )
		maxtocopy = qsize;
	memcpy( data, queue->qData, maxtocopy );
	*psize = maxtocopy;
	
	// free up the queue
	queue->Remove( netQueue );
	delete[] reinterpret_cast<char *>( queue );
	
	return noErr;
}


/*
**	DTSNetChannelPriv::AddToHash()
**
**	[server]
**	add the channel to the hash table
*/
DTSError
DTSNetChannelPriv::AddToHash( DTSNetChannel * channel )
{
	uint hash = channel->priv.p->HashFunction();
	uint sz = 1U << netHostMax;
	DTSHostHash * start = netHostHash;
	DTSHostHash * table = start + hash;
	
	for ( uint index = sz;  index > 0;  --index )
		{
		if ( hash++ == sz )
			table = start;
		
		DTSNetChannel * test = table->hhChannel;
		if ( not test )
			{
			// record address
			table->hhAddress = channel->priv.p->netDstAddr;
			table->hhChannel = channel;
			return noErr;
			}
		
		++table;
		}
	
	return -1;		// table full
}


/*
**	DTSNetChannelPriv::RemoveFromHash()
**
**	[server]
**	remove the channel from the hash table
*/
void
DTSNetChannelPriv::RemoveFromHash()
{
	DTSNetChannelPriv * host = netHostChannel;
	if ( not host )
		return;
	
	uint hash = HashFunction();
	uint sz = 1U << host->netHostMax;
	DTSHostHash * start = host->netHostHash;
	DTSHostHash * table = start + hash;
	
	for ( uint index = sz;  index > 0;  --index )
		{
		if ( hash++ == sz )
			table = start;
		
		const DTSNetChannel * test = table->hhChannel;
		if ( test  &&  test->priv.p == this )
			{
			table->hhChannel = nullptr;
			break;
			}
		
		++table;
		}
}


/*
**	DTSNetChannelPriv::FindInHash()
**
**	[server]
**	return the channel having a given address, if any
*/
DTSError
DTSNetChannelPriv::FindInHash( const DTSNetAddress& srcaddr, DTSNetChannel ** pchannel )
{
	int shift = netHostMax;
	uint hash = PrivHashFunction( srcaddr, shift );
	uint sz = 1U << shift;
	const DTSHostHash * start = netHostHash;
	const DTSHostHash * table = start + hash;
	DTSNetChannel * channel = nullptr;
	DTSError result = -1;
	
	for ( uint index = sz;  index > 0;  --index )
		{
		if ( hash++ == sz )
			table = start;
		
		// do the port and IP address match?
		if ( table->hhAddress == srcaddr )
			{
			channel = table->hhChannel;
			if ( channel )
				result = noErr;
			break;
			}
		
		++table;
		}
	
	*pchannel = channel;
	return result;
}


/*
**	DTSNetChannelPriv::HashFunction()
**
**	[server]
**	hash up the IP address to get an index into the table
*/
uint
DTSNetChannelPriv::HashFunction() const
{
	if ( const DTSNetChannelPriv * host = netHostChannel )
		{
		uint hash = PrivHashFunction( netDstAddr, host->netHostMax );
		return hash;
		}
	
	return 0;
}


/*
**	PrivHashFunction()
**
**	[server]
**	hash up the IP address to get an index into the table
*/
uint
PrivHashFunction( const DTSNetAddress& addr, int shift )
{
	// port number
	uint hash = addr.Port();
	
	// IPv4 address, in two 16-bit halves
	in_addr_t ip4 = addr.IPAddr();
	uint16_t * ip = reinterpret_cast<uint16_t *>( &ip4 );
	hash += ip[ 0 ];
	hash += ip[ 1 ];
	
	uint mask = (1U << shift) - 1;
	
	// fold in any bits beyond the shift-size limit
	while ( hash & ~mask )
		hash = ( hash & mask ) + ( hash >> shift );
	
	return hash;
}


/*
**	DTSNetChannelPriv::FreeQueued()
**
**	[server]
**	free all elements in the queue linked list
*/
void
DTSNetChannelPriv::FreeQueued()
{
	DTSQueue * next;
	for ( DTSQueue * queue = netQueue;  queue;  queue = next )
		{
		next = queue->linkNext;
		delete[] reinterpret_cast<char *>( queue );
		}
	netQueue = nullptr;
}


/*
**	DTSNetChannelPriv::GetPeerAddress()
**
**	[server]
**	returns address of remote peer, if we know it
*/
DTSError
DTSNetChannelPriv::GetPeerAddress( DTSNetAddress& oAddr ) const
{
	if ( netConnected )
		{
		// send back the peer's address
		oAddr = netDstAddr;
		
		return noErr;
		}
	
	return kNetworkPortNotAvailable;	// ??
}

