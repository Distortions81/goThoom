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

#include <string.h>
#include <stdlib.h>
#include <OpenTransport.h>

// brain damage
#include "DTS.h" // just for GetFrameCounter!

#include "LinkedList_dts.h"
#include "Network_dts.h"
#include "Network_cmn.h"


/*
**	hack for slipping tcp/udp file descriptors into and out of this file
*/
#ifdef DTS_Unix
int handofftcp;
int handoffudp;
#endif


/*
**	Entry Routines
**
**	short	DTSInitNetwork();
**	void	DTSExitNetwork();
**	long	DTSEthernetHardwareAddress();
**
**	class DTSNetChannelPriv;
*/


/*
**	Protocol Documentation:
**
**	for this example:
**	the server's well known tcp and udp port numbers are both 5000
**	arbitrary addresses are 8000+
**
**	server opens tcp5000 and udp5000
**	server opens tcp8000
**
**	client opens tcp8000 and udp8000
**	client connects to server tcp5000
**
**	server accepts on tcp8000
**	server generates a unique identifier for this client
**	server sends identifier to client on tcp8000
**
**	client sends a special flag followed by the identifier back to the server at udp5000
**	client retries twice more if necessary
**
**	server stores the client udp address
**	server sends on tcp8000 a message informing the client we are now fully connected
**
**	client and server can now exchange messages through a paranoid firewall
**
**	server sends udp packets to all clients through udp5000
**	all clients send udp packets to the server at udp5000
**	when the server receives a packet on udp5000 it hashes up the from address to help
**	find the proper channel
**	that packet is added to the channel's queue
*/


/*
**	Transport Documentation
**
**	In theory these routines can generate these look events:
**	In practice we never connect on any endpoint that is listening.
**	OTAccept							T_DISCONNECT, T_LISTEN
**	OTConnect							T_DISCONNECT, {T_LISTEN}
**	OTListen, OTRcvConnect,				T_DISCONNECT
**	OTRcvOrderlyDisconnect,				T_DISCONNECT
**	OTSndOrderlyDisconnect,				T_DISCONNECT
**	OTSndDisconnect						T_DISCONNECT
**	OTRcv, OTRcvRequest, OTRcvReply,	T_DISCONNECT, T_ORDREL
**	OTSnd, OTSndRequest, OTSndReply		T_DISCONNECT, T_ORDREL
**	OTRcvUData, OTSndUData				T_UDERR
**	OTUnbind							T_LISTEN, T_DATA
**
**	These look events will be cleared by these routines:
**	T_LISTEN			OTListen
**	T_CONNECT			OTRcvConnect
**	T_DATA				OTRcv, OTRcvUData
**	T_EXDATA			OTRcv
**	T_DISCONNECT		OTRcvDisconnect
**	T_UDERR				OTRcvUDErr
**	T_ORDREL			OTRcvOrderlyDisconnect
**	T_GODATA			OTSnd, OTSndUData
**	T_GOEXDATA			OTSnd
*/


/*
**	Definitions
*/
#ifdef DTS_Mac
#define		kClosedEndPoint			(nil)
#define		IsEndPointOpen(ep)		((ep)!=nil)
typedef		TCall					DTSNetCall;
#else
#define		kClosedEndPoint			(-1)
#define		IsEndPointOpen(ep)		((ep)>=0)
typedef		struct t_call			DTSNetCall;
#endif

#define		kHandshakeTimeout		(10*60)		// 10 seconds
#define		kHandshakeTag			((short)0xFFFF)

#define		TIMEOUT					2000

class DTSConnectList : public DTSLinkedList<DTSConnectList>
	{
public:
	long				clState;
	DTSNetChannelPriv *		clChannel;
	DTSNetCall *		clCall;
	ulong				clUDPTimeOut;
	long				clUniqueID;
	
	// constructor/deconstructor
	DTSConnectList( void );
	virtual ~DTSConnectList( void );
	};

class DTSQueue
	{
public:
	DTSQueue *	qNext;
	long		qSize;
	char		qData[2];
	
	// constructor/destructor
	DTSQueue( void ) {};
	~DTSQueue( void ) {};
	};

class DTSHostHash
	{
public:
	uchar				hhAddr[6];
	DTSNetChannelPriv *		hhChannel;
	};

enum
	{
	kConnectNotInited,
	kConnectReadyToListen,
	kConnectReadyToAccept,
	kConnectReadyToSendUDP,
	kConnectWaitForUDP,
	kConnectSendConfirm,
	kConnectConnected
	};

enum
	{
	kNetworkLook				= -31999,
	kNetworkListen				= -31998,
	kNetworkDisconnect			= -31997,
	kNetworkUnknownLookErr		= -31996,
	kNetworkHandshakeTimeout	= -31995,
	kNetworkPartialSend			= -31994,
	kNetworkUnitData			= -31993,
	kNetworkPortNotAvailable	= -31992,
	kNetworkBadTCPAddress       = -31991,
	
	kNetworkNoData				= +31999
	};


/*
**	Internal Routines
*/
static	long	PrivHashFunction( uchar * addr, short shift );
static	short	TCPStringToAddr( DTSEndPoint ep, char * str, uchar * addr );
static	short	DTSNetOpenTCP( DTSEndPoint * ep, short * port );
static	short	DTSNetOpenUDP( DTSEndPoint * ep, short * port );
static	short	DTSNetOpen( DTSEndPoint * ep, short * port, char * path );
static	void	DTSNetClose( DTSEndPoint ep );
static	short	DTSNetAlloc( DTSEndPoint ep, DTSNetCall ** call );
static	void	DTSNetFree( DTSNetCall * call );
static	short	DTSNetListen( DTSEndPoint ep, DTSNetCall * call );
static	short	DTSNetAccept( DTSEndPoint hostep, DTSEndPoint chanep, DTSNetCall * call );
static	short	DTSNetSend( DTSEndPoint ep, void * data, short size );
static	short	DTSNetReceive( DTSEndPoint ep, void * data, short * size );
static	short	DTSNetUSend( DTSEndPoint ep, uchar * toaddr, void * data, short size );
static	short	DTSNetUReceive( DTSEndPoint ep, uchar * fromaddr, void * data, short * size );
static	short	DTSNetConnect( DTSEndPoint ep, uchar * hostaddr, DTSNetChannelPriv * channel );
static	short	DTSNetResolveName( DTSEndPoint ep, char * str, uchar * addr );


/*
**	Internal Variables
*/
static	long		gUniqueID;


/*
**	DTSNetChannelPriv::DTSNetChannelPriv()
**
**	constructor
*/
DTSNetChannelPriv::DTSNetChannelPriv()
{
	netTCP         = kClosedEndPoint;
	netUDP         = kClosedEndPoint;
	netConnected   = false;
	netHostChannel = nil;
	netQueue       = nil;
	netConnectList = nil;
	netHostHash    = nil;
	netHostMax     = 0;
	netPort        = 0;
	memset( netDstIPAddr, 0, sizeof(netDstIPAddr) );
}


/*
**	DTSNetChannelPriv::~DTSNetChannelPriv()
**
**	deconstructor
*/
DTSNetChannelPriv::~DTSNetChannelPriv()
{
	Close();
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
	// hack for handing off end points
#ifdef DTS_Unix
	if ( handofftcp == netTCP )
		handofftcp = 0;
	if ( handoffudp == netUDP )
		handoffudp = 0;
#endif
	
	RemoveFromHash();
	DTSNetClose( netTCP );
	DTSNetClose( netUDP );
	FreeQueued();
	DTSConnectList::DeleteLinkedList( netConnectList );
	delete[] (char *) netHostHash;
	
	netTCP         = kClosedEndPoint;
	netUDP         = kClosedEndPoint;
	netConnected   = false;
	netHostChannel = nil;
	netQueue       = nil;
	netConnectList = nil;
	netHostHash    = nil;
	netHostMax     = 0;
	netPort        = 0;
	memset( netDstIPAddr, 0, sizeof(netDstIPAddr) );
}


/*
**	DTSNetChannelPriv::IsConnected()
**
**	return true if connected
*/
DTSBoolean
DTSNetChannelPriv::IsConnected()
{
	return netConnected;
}


/*
**	DTSNetChannelPriv::InitHost()
**
**	initialize a well known tcp port for listening
*/
short
DTSNetChannelPriv::InitHost( short port, short max )
{
	DTSEndPoint tcpep = kClosedEndPoint;
	DTSEndPoint udpep = kClosedEndPoint;
	DTSHostHash * hash = nil;
	long count;
	long size;
	
	short result = noErr;
	// if ( noErr == result )
		{
		// hack for handing off end points
#ifdef DTS_Unix
		if ( handofftcp == 0 )
			result = DTSNetOpenTCP( &tcpep, &port );
		else
			tcpep = handofftcp;
#else
		result = DTSNetOpenTCP( &tcpep, &port );
#endif
		}
	if ( noErr == result )
		{
		// hack for handing off end points
#ifdef DTS_Unix
		if ( handoffudp == 0 )
			result = DTSNetOpenUDP( &udpep, &port );
		else
			udpep = handoffudp;
#else
		result = DTSNetOpenUDP( &udpep, &port );
#endif
		}
	if ( noErr == result )
		{
		count = 1;
		for ( size = 1;  size < max;  size += size + 1 )
			{
			++count;
			}
		size *= sizeof(DTSHostHash);
		
		hash = (DTSHostHash *) NEW_TAG("DTSNetHash") char[size];
		if ( nil == hash )
			{
			result = memFullErr;
			}
		}
	if ( noErr == result )
		{
		memset( hash, 0, size );
		
		netTCP      = tcpep;
		netUDP      = udpep;
		netPort     = port;
		netHostHash = hash;
		netHostMax  = count;
		
		// hack for handing off end points
#ifdef DTS_Unix
		handofftcp = tcpep;
		handoffudp = udpep;
#endif
		}
	
	if ( noErr != result )
		{
		DTSNetClose( tcpep );
		DTSNetClose( udpep );
		delete[] (char *) hash;
		}
	
	return result;
}


/*
**	DTSNetChannelPriv::AnswerClient()
**
**	listen for a client to connect
*/
short
DTSNetChannelPriv::AnswerClient( DTSNetChannelPriv ** pchannel )
{
	DTSConnectList * connect = nil;
	DTSNetChannelPriv * channel = nil;
	DTSNetChannelPriv * test;
	
	short result = noErr;
	// if ( noErr == result )
		{
		connect = FindConnection( kConnectConnected );
		}
	if ( noErr == result )
		{
		result = HandleUDP();
		}
	if ( noErr == result )
		{
		result = HandleConnectList();
		}
	if ( connect )
		{
		if ( noErr == result )
			{
			test                 = connect->clChannel;
			test->netHostChannel = this;
			result = AddToHash( test );
			}
		if ( noErr == result )
			{
			channel            = test;
			connect->clChannel = nil;
			connect->Remove( netConnectList );
			delete connect;
			}
		}
	
	*pchannel = channel;
	return result;
}


/*
**	DTSNetChannelPriv::FindConnection()
**
**	find a connection in the state
*/
DTSConnectList *
DTSNetChannelPriv::FindConnection( short state )
{
	for ( DTSConnectList * connect = netConnectList;  connect;  connect = connect->linkNext )
		{
		if ( connect->clState == state )
			{
			return connect;
			}
		}
	
	return nil;
}


/*
**	DTSNetChannelPriv::HandleConnectList()
**
**	progress all of the pending connections
*/
short
DTSNetChannelPriv::HandleConnectList()
{
	DTSBoolean bMoreToDo;
	do
		{
		bMoreToDo = false;
		
		// make sure someone is listening
		DTSConnectList * connect = FindConnection( kConnectReadyToListen );
		if ( connect == nil )
			{
			short result = CreateConnect();
			if ( result != noErr )
				{
				// failure here would be really bad!
				return result;
				}
			}
		
		// do the periodic task for every pending connection
		DTSConnectList * next;
		for ( connect = netConnectList;  connect;  connect = next )
			{
			next = connect->linkNext;
			DTSBoolean bDoMore = Handle1Connect( connect );
			if ( bDoMore )
				{
				bMoreToDo = true;
				}
			}
		}
	while ( bMoreToDo );
	
	return noErr;
}


/*
**	DTSNetChannelPriv::CreateConnect()
**
**	create a new connection ready to listen
*/
short
DTSNetChannelPriv::CreateConnect()
{
	DTSConnectList * connect = nil;
	DTSNetChannelPriv * channel = nil;
	DTSEndPoint tcpep = kClosedEndPoint;
	DTSNetCall * call = nil;
	
	short result = noErr;
	// if ( noErr == result )
		{
		connect = NEW_TAG("DTSConnectList") DTSConnectList;
		if ( nil == connect )
			{
			result = memFullErr;
			}
		}
	if ( noErr == result )
		{
		channel = NEW_TAG("DTSNetChannelPriv") DTSNetChannelPriv;
		if ( nil == channel )
			{
			result = memFullErr;
			}
		}
	if ( noErr == result )
		{
		result = DTSNetOpenTCP( &tcpep, nil );
		}
	if ( noErr == result )
		{
		result = DTSNetAlloc( tcpep, &call );
		}
	if ( noErr == result )
		{
		channel->netTCP    = tcpep;
		connect->clState   = kConnectReadyToListen;
		connect->clChannel = channel;
		connect->clCall    = call;
		connect->InstallFirst( netConnectList );
		}
	
	if ( noErr != result )
		{
		delete connect;
		delete channel;
		DTSNetClose( tcpep );
		DTSNetFree( call );
		}
	
	return result;
}


/*
**	DTSNetChannelPriv::Handle1Connect()
**
**	progress one pending connections
*/
DTSBoolean
DTSNetChannelPriv::Handle1Connect( DTSConnectList * connect )
{
	DTSBoolean bMoreToDo = false;
	
	short state = connect->clState;
	switch ( state )
		{
		// ready to listen
		case kConnectReadyToListen:
			bMoreToDo = DoListenTask( connect );
			break;
		
		// ready to accept
		case kConnectReadyToAccept:
			bMoreToDo = DoAcceptTask( connect );
			break;
		
		// ready to send the upd address
		case kConnectReadyToSendUDP:
			bMoreToDo = DoSendUDPTask( connect );
			break;
		
		// wait for the udp to connect
		case kConnectWaitForUDP:
			bMoreToDo = DoWaitUDPTask( connect );
			break;
		
		// send the confirmation on tcp
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


/*
**	DTSNetChannelPriv::DoListenTask()
**
**	listen for connections
*/
DTSBoolean
DTSNetChannelPriv::DoListenTask( DTSConnectList * connect )
{
	// listen 
	short result = DTSNetListen( netTCP, connect->clCall );
	
	// no one is calling
	// no more to do
	if ( result > 0 )
		{
		return false;
		}
	
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
**	accept a connection
*/
DTSBoolean
DTSNetChannelPriv::DoAcceptTask( DTSConnectList * connect )
{
	// accept
	DTSEndPoint hostep = netTCP;
	DTSNetChannelPriv * channel = connect->clChannel;
	short result = DTSNetAccept( hostep, channel->netTCP, connect->clCall );
	
	// success, change to the next state
	// and there is more to do
	if ( noErr == result )
		{
		connect->clState = kConnectReadyToSendUDP;
		return true;
		}
	
	// listen event, stay in this state
	// and there's more to do
	// specifically, someone needs to listen
	// and we need to accept again
	// (timmmer thinks this is a rather stupid interface)
	if ( kNetworkListen == result )
		{
		return true;
		}
	
	// some other error
	// give up
	connect->Remove( netConnectList );
	delete connect;
	return false;
}


/*
**	DTSNetChannelPriv::DoSendUDPTask()
**
**	send the client their unique identifier
*/
DTSBoolean
DTSNetChannelPriv::DoSendUDPTask( DTSConnectList * connect )
{
	// send the unique identifier to the client on the tcp endpoint
	DTSNetChannelPriv * channel = connect->clChannel;
	short result = DTSNetSend( channel->netTCP, &connect->clUniqueID, sizeof(connect->clUniqueID) );
	
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
	connect->clUDPTimeOut = GetFrameCounter() + 3*kHandshakeTimeout;
	
	// we're done for now
	return false;
}


/*
**	DTSNetChannelPriv::DoWaitUDPTask()
**
**	wait for the client to send their unique identifier via udp
*/
DTSBoolean
DTSNetChannelPriv::DoWaitUDPTask( DTSConnectList * connect )
{
	// we handle the timeout here
	// the change of state happens in Handle1NewUDP when
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
**	progress one pending connections
*/
DTSBoolean
DTSNetChannelPriv::DoConfirmTask( DTSConnectList * connect )
{
	// send the confirmation data
	short result = noErr;
	// if ( noErr == result )
		{
		char buff[2];
		buff[0] = 0xFF;
		buff[1] = 0xFF;
		DTSNetChannelPriv * channel = connect->clChannel;
		result = DTSNetSend( channel->netTCP, buff, sizeof(buff) );
		}
	// we are now fully connected
	if ( noErr == result )
		{
		connect->clState = kConnectConnected;
		return false;
		}
	
	// check for timeout
	ulong timeout = connect->clUDPTimeOut;
	ulong curTime = GetFrameCounter();
	if ( curTime > timeout )
		{
		result = kNetworkHandshakeTimeout;
		}
	
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
**	constructor
*/
DTSConnectList::DTSConnectList()
{
	// update the unique identifier
	// in order to completely avoid accidentally establishing
	// a connection with an older and incompatible version of this library...
	// which used to send a two byte port number...
	// we make sure the top two bytes of this unique id are not in the range
	// used by any application we have shipped so far.
	// clan lord used ports near and above 5000
	// skip everything up to 5099
	// just to be paranoidally safe.
	long id      = gUniqueID + 1;
	long oldport = ( id >> 16 ) & 0x0000FFFF;
	if ( oldport >= 5000
	&&   oldport <= 5099 )
		{
		id = ( 5100 << 16 ) + ( id & 0x0000FFFF );
		}
	gUniqueID    = id;
	
	clState      = kConnectNotInited;
	clChannel    = nil;
	clCall       = nil;
	clUDPTimeOut = 0;
	clUniqueID   = id;
}


/*
**	DTSConnectList::~DTSConnectList()
**
**	destructor
*/
DTSConnectList::~DTSConnectList()
{
	delete clChannel;
	DTSNetFree( clCall );
}


/*
**	DTSNetChannelPriv::ConnectHost()
**
**	connect to the host with the given address
*/
short
DTSNetChannelPriv::ConnectHost( char * hoststring, short portin )
{
	// handle if the client requested a specific port
	short * pport = nil;
	if ( portin )
		pport = &portin;
	
	short result = noErr;
	// if ( noErr == result )
		{
		result = DTSNetOpenTCP( &netTCP, pport );
		}
	if ( noErr == result )
		{
		result = DTSNetOpenUDP( &netUDP, pport );
		}
	if ( noErr == result )
		{
		result = TCPStringToAddr( netTCP, hoststring, netDstIPAddr );
		}
	if ( noErr == result )
		{
		result = ConnectTCP();
		}
	if ( noErr == result )
		{
		result = ConnectUDP();
		}
	if ( noErr == result )
		{
		netConnected = true;
		}
	
	return result;
}


/*
**	DTSNetChannelPriv::ConnectTCP()
**
**	establish the tcp connection
*/
short
DTSNetChannelPriv::ConnectTCP()
{
	DTSEndPoint ep = netTCP;
	short result = noErr;
	// if ( noErr == result )
		{
		result = ConnectCallBack( "Connecting to host." );
		}
	if ( noErr == result )
		{
		result = DTSNetConnect( ep, netDstIPAddr, this );
		}
	if ( noErr == result )
		{
		result = ConnectCallBack( "Established reliable connection." );
		}
	
	return result;
}


/*
**	DTSNetChannelPriv::ConnectUDP()
**
**	establish the udp connection
*/
short
DTSNetChannelPriv::ConnectUDP()
{
	long id;
	
	// wait for the unique identifier to arrive on the tcp endpoint
	short result = noErr;
	// if ( noErr == result )
		{
		result = ConnectCallBack( "Waiting for unreliable connection from host." );
		}
	if ( noErr == result )
		{
		result = ConnectUDPWait( &id );
		}
	// great now we have the unique identifier
	// send it back to the server on the udp port
	// wait for confirmation on the tcp port
	if ( noErr == result )
		{
		result = ConnectCallBack( "Establishing unreliable connection." );
		}
	if ( noErr == result )
		{
		result = ConnectUDPExchange( id );
		}
	// first retry
	if ( result == kNetworkHandshakeTimeout )
		{
		result = ConnectCallBack( "First unreliable connection retry." );
		if ( noErr == result )
			{
			result = ConnectUDPExchange( id );
			}
		}
	// second and last retry
	if ( result == kNetworkHandshakeTimeout )
		{
		result = ConnectCallBack( "Second and last unreliable connection retry." );
		if ( noErr == result )
			{
			result = ConnectUDPExchange( id );
			}
		}
	if ( noErr == result )
		{
		result = ConnectCallBack( "Unreliable connection established." );
		}
	
	return result;
}


/*
**	DTSNetChannelPriv::ConnectUDPWait()
**
**	wait for the unique identifier to arrive on the tcp endpoint
*/
short
DTSNetChannelPriv::ConnectUDPWait( long * id )
{
	// wait for our unique identifier to arrive on the tcp endpoint
	ulong stopTime = GetFrameCounter() + kHandshakeTimeout;
	short result;
	DTSEndPoint tcpep = netTCP;
	for(;;)
		{
		// receive our unique identifier
		short size = sizeof(*id);
		result = DTSNetReceive( tcpep, id, &size );
		
		// bail on success
		// bail on hard error
		if ( noErr >= result )
			{
			break;
			}
		
		// check for timeout
		ulong curTime = GetFrameCounter();
		if ( curTime > stopTime )
			{
			result = kNetworkHandshakeTimeout;
			break;
			}
		
		// call the call back routine
		result = ConnectCallBack( nil );
		if ( noErr != result )
			{
			break;
			}
		}
	
	return result;
}


/*
**	DTSNetChannelPriv::ConnectUDPExchange()
**
**	send the unique id to the server on the udp port
**	wait for confirmation on the tcp port
*/
short
DTSNetChannelPriv::ConnectUDPExchange( long id )
{
	// great we now have a unique identifier
	// send it back to the server
	short result = noErr;
	// if ( noErr == result )
		{
		// since the listener is expecting complete packets to arrive
		// on the udp port we had better send it something it can
		// at least recognize as a packet.
		// we set the "size" to be 0xFFFF
		// and after that we send 4 bytes with the unique identifier
		// the server doesn't know about us yet.
		// and it ignores packets from unknown senders
		// unless they are in this format.
		// in which case it will remember us.
		uchar buff[6];
		buff[0] = 0xFF;
		buff[1] = 0xFF;
		buff[2] = id >> 24;
		buff[3] = id >> 16;
		buff[4] = id >>  8;
		buff[5] = id >>  0;
		
		result = DTSNetUSend( netUDP, netDstIPAddr, buff, sizeof(buff) );
		}
	
	// wait for the confirmation to arrive on the tcp channel
	if ( noErr == result )
		{
		ulong stopTime = GetFrameCounter() + kHandshakeTimeout;
		DTSEndPoint tcpep = netTCP;
		for(;;)
			{
			// receive the confirmation
			short dummyData;
			short size = sizeof(dummyData);
			result = DTSNetReceive( tcpep, &dummyData, &size );
			
			// bail on success
			// bail on hard error
			if ( noErr >= result )
				{
				break;
				}
			
			// check for timeout
			ulong curTime = GetFrameCounter();
			if ( curTime > stopTime )
				{
				result = kNetworkHandshakeTimeout;
				break;
				}
			
			// call the call back routine
			result = ConnectCallBack( nil );
			if ( noErr != result )
				{
				break;
				}
			}
		}
	
	return result;
}


/*
**	DTSNetChannelPriv::ConnectCallBack()
**
**	the default call back routine does nothing
*/
short
DTSNetChannelPriv::ConnectCallBack( char * status )
{
	#pragma unused( status )
	return noErr;
}


/*
**	DTSNetChannelPriv::Write()
**
**	write data
*/
short
DTSNetChannelPriv::Write( const void * data, short size, short type )
{
	// clear udp packets from the host channel
	HandleHostQueue();
	
	char * ptr = nil;
	short msgsize = size + sizeof(size);
	
	// convert the raw data to a message
	short result = noErr;
	// if ( noErr == result )
		{
		ptr = NEW_TAG("DTSNetWriteTemp") char[ msgsize ];
		if ( nil == ptr )
			{
			result = memFullErr;
			}
		}
	// send the messaage reliably with tcp
	// or unreliably with udp
	if ( noErr == result )
		{
		ptr[0] = size >> 8;
		ptr[1] = size;
		memcpy( ptr + 2, data, size );
		
		if ( type == kNetWriteReliable )
			{
			result = DTSNetSend( netTCP, ptr, msgsize );
			}
		else
			{
			DTSEndPoint ep = netUDP;
			if ( ! IsEndPointOpen(ep) )
				{
				DTSNetChannelPriv * host = netHostChannel;
				if ( host )
					{
					ep = host->netUDP;
					}
				}
			if ( IsEndPointOpen(ep) )
				{
				result = DTSNetUSend( ep, netDstIPAddr, ptr, msgsize );
				}
			else
				{
				result = -1;
				}
			}
		}
	
	// track disconnects
	if ( kNetworkDisconnect == result )
		{
		netConnected = false;
		}
	
	delete[] ptr;
	
	return result;
}


/*
**	DTSNetChannelPriv::Read()
**
**	read data
*/
short
DTSNetChannelPriv::Read( void * data, short * psize )
{
	// clear udp packets from the host channel
	HandleHostQueue();
	
	// read from the reliable tcp endpoint
	short maxtoread = *psize;
	short result = netTCPPacket.ReadData( netTCP, nil, data, psize );
	
	// read unreliable data
	if ( noErr < result )
		{
		// read from a client's unreliable udp endpoint;
		DTSEndPoint ep = netUDP;
		*psize = maxtoread;
		if ( IsEndPointOpen(ep) )
			{
			result = netUDPPacket.ReadData( ep, netDstIPAddr, data, psize );
			}
		// or read from a host's queue
		else
			{
			result = ReadNetQueue( data, psize );
			}
		}
	
	// track disconnects
	if ( kNetworkDisconnect == result )
		{
		netConnected = false;
		}
	
	return result;
}


/*
**	PartialPacket::PartialPacket()
**
**	constructor
*/
PartialPacket::PartialPacket()
	:	ppBuffer(nil), ppMaxSize(0), ppSize(0), ppRead(0)
	{}


/*
**	PartialPacket::~PartialPacket()
**
**	constructor
*/
PartialPacket::~PartialPacket()
{
	delete[] ppBuffer;
}


/*
**	PartialPacket::Allocate()
**
**	allocate memory for the packet
*/
short
PartialPacket::Allocate()
{
	// replace the buffer if it is too small
	short result = noErr;
	short maxSize = ppMaxSize;
	short size    = ppSize;
	
	// check the size of the buffer against the size needed
	if ( maxSize < size )
		{
		// allocate a new buffer
		uchar * newbuff = NEW_TAG("DTSNetPacket") uchar[ size ];
		
		// bail if failed
		if ( newbuff == nil )
			{
			result = memFullErr;
			}
		// delete the old buffer
		// save the new one and its size
		else
			{
			delete[] ppBuffer;
			ppBuffer  = newbuff;
			ppMaxSize = size;
			}
		}
	
	return result;
}


/*
**	PartialPacket::ReadData()
**
**	return complete data from the packet and reset
*/
short
PartialPacket::ReadData( DTSEndPoint ep, uchar * unreliableAddr, void * data, short * size )
{
	short numread = 0;
	
	// check the partial buffer
	short result = noErr;
	// if ( noErr == result )
		{
		result = FillPartialBuffer( ep, unreliableAddr );
		}
	// copy the data from the partial buffer
	if ( noErr == result )
		{
		// pin to the size of the destination
		numread = *size;
		short numinbuff = ppSize;
		if ( numread >= numinbuff )
			{
			numread = numinbuff;
			}
		memcpy( data, ppBuffer, numread );
		
		// reset the partial packet
		Reset();
		}
	
	*size = numread;
	return result;
}


/*
**	PartialPacket::FillPartialBuffer()
**
**	fill the partial buffer
*/
short
PartialPacket::FillPartialBuffer( DTSEndPoint ep, uchar * unreliableAddr )
{
	short result = noErr;
	short count;
	char buff[4];
	
	// are we starting a new packet?
	if ( ppSize == 0 )
		{
		// read the size of the packet
		// if ( noErr == result )
			{
			count = sizeof(short);
			if ( unreliableAddr )
				{
				result = DTSNetUReceive( ep, unreliableAddr, buff, &count );
				}
			else
				{
				result = DTSNetReceive( ep, buff, &count );
				}
			}
		// allocate memory for the packet
		if ( noErr == result )
			{
			// check the packet size for the special handshake value
			// and ignore it
			count = ( ((uchar) buff[0]) << 8 ) + ((uchar) buff[1]);
			if ( kHandshakeTag == count )
				{
				result = 1;
				}
			else
				{
				ppSize = count;
				result = Allocate();
				}
			}
		}
	
	// attempt to read data
	if ( noErr == result )
		{
		// check if the packet is partially full
		short numRead   = ppRead;
		short numToRead = ppSize - numRead;
		if ( numToRead > 0 )
			{
			// if ( noErr == result )
				{
				count = numToRead;
				uchar * dest = ppBuffer + numRead;
				if ( unreliableAddr )
					{
					result = DTSNetUReceive( ep, unreliableAddr, dest, &count );
					}
				else
					{
					result = DTSNetReceive( ep, dest, &count );
					}
				}
			if ( noErr == result )
				{
				ppRead = numRead + count;
				}
			}
		}
	
	// change the error code if the packet is not yet full
	if ( noErr == result )
		{
		if ( ppSize > ppRead )
			{
			result = kNetworkNoData;
			}
		}
	
	return result;
}


/*
**	PartialPacket::Reset()
**
**	reset the partial packet
**	so it's ready to read a new packet
*/
void
PartialPacket::Reset()
{
	ppSize = 0;
	ppRead = 0;
}


/*
**	TCPStringToAddr()
**
**	convert a string "123.45.67.8:5000"
**	to the tcp address the transport layer is looking for
*/
static short
TCPStringToAddr( DTSEndPoint ep, char * str, uchar * addr )
{
	// check for a string of format "123.45.67.8:5000"
	long n1;
	long n2;
	long n3;
	long n4;
	long port;
	int num = sscanf( str, "%ld.%ld.%ld.%ld:%ld", &n1, &n2, &n3, &n4, &port );
	if ( 5 == num )
		{
		memset( addr, 0, 16 );
		addr[1] = 2;
		addr[2] = port >> 8;
		addr[3] = port;
		addr[4] = n1;
		addr[5] = n2;
		addr[6] = n3;
		addr[7] = n4;
		
		return noErr;
		}
	
	// check for a domain name with a port number
	char * colon = strrchr( str, ':' );
	if ( colon )
		{
		port = atol( colon + 1 );
		*colon = 0;
		}
	
	// resolve just the domain name
	// be paranoid
	long result = noErr;
	// if ( noErr == result )
		{
		result = DTSNetResolveName( ep, str, addr );
		}
	
	// fold the port # back in, if any
	if (  colon )
		{
		*colon = ':';
		if ( noErr == result )
			{
			addr[2] = port >> 8;
			addr[3] = port;
			}
		}
	
	return result;
}


/*
**	DTSNetChannelPriv::HandleHostQueue()
**
**	read udp packets from the host channel
*/
short
DTSNetChannelPriv::HandleHostQueue()
{
	DTSNetChannelPriv * host = netHostChannel;
	if ( nil == host )
		{
		return noErr;
		}
	
	short result = host->HandleUDP();
	
	return result;
}


/*
**	DTSNetChannelPriv::HandleUDP()
**
**	read udp packets from the host channel
*/
short
DTSNetChannelPriv::HandleUDP()
{
	short result;
	for(;;)
		{
		result = Handle1UDP();
		if ( kNetworkNoData == result )
			{
			result = noErr;
			break;
			}
		if ( noErr > result )
			{
			break;
			}
		}
	
	return result;
}


/*
**	DTSNetChannelPriv::Handle1UDP()
**
**	read 1 udp packet from the host channel
*/
short
DTSNetChannelPriv::Handle1UDP()
{
	short count;
	char buff[4];
	uchar srcaddr[16];
	
	short result = noErr;
	// if ( noErr == result )
		{
		count = sizeof(short);
		memset( srcaddr, 0, sizeof(srcaddr) );
		result = DTSNetUReceive( netUDP, srcaddr, buff, &count );
		}
	if ( noErr == result )
		{
		if ( sizeof(short) != count )
			{
			// we only read one byte for a two byte count?
			// we might be hosed.
			// but we might miraculously be okay
			// return a soft error
			result = +1;
			}
		}
	if ( noErr == result )
		{
		count = ( ((uchar) buff[0]) << 8 ) + ((uchar) buff[1]);
		if ( count == kHandshakeTag )
			{
			result = Handle1NewUDP( srcaddr );
			}
		else
		if ( count > 0 )
			{
			result = Handle1ExistingUDP( srcaddr, count );
			}
		else
			{
			// count is negative?
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
**	this packet is from a newly connecting client
*/
short
DTSNetChannelPriv::Handle1NewUDP( uchar * srcaddr )
{
	char buff[4];
	short count;
	
	short result = noErr;
	// if ( noErr == result )
		{
		count = sizeof(buff);
		result = DTSNetUReceive( netUDP, srcaddr, buff, &count );
		}
	if ( noErr == result )
		{
		if ( sizeof(buff) != count )
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
		long id = ( ((uchar) buff[0]) << 24 ) 
		        + ( ((uchar) buff[1]) << 16 )
		        + ( ((uchar) buff[2]) <<  8 )
		        + ( ((uchar) buff[3]) <<  0 );
		
		for( DTSConnectList * connect = netConnectList;  ;  connect = connect->linkNext )
			{
			if ( nil == connect )
				{
				// connection not found in the pending list
				// just ignore it?
				break;
				}
			if ( connect->clState    == kConnectWaitForUDP
			&&   connect->clUniqueID == id )
				{
				connect->clState = kConnectSendConfirm;
				DTSNetChannelPriv * channel = connect->clChannel;
				memcpy( channel->netDstIPAddr, srcaddr, sizeof(channel->netDstIPAddr) );
				break;
				}
			}
		}
	
	return result;
}


/*
**	DTSNetChannelPriv::Handle1ExistingUDP()
**
**	read 1 udp packet from the host channel
**	stuff it into the appropriate client channel
*/
short
DTSNetChannelPriv::Handle1ExistingUDP( uchar * srcaddr, short count )
{
	DTSQueue * queue = nil;
	DTSNetChannelPriv * channel = nil;
	long size;
	short numread;
	
	short result = noErr;
	// if ( noErr == result )
		{
		size = sizeof(*queue) - sizeof(queue->qData) + count;
		queue = (DTSQueue *) NEW_TAG("DTSNetQueue") char[size];
		if ( nil == queue )
			{
			result = memFullErr;
			}
		}
	if ( noErr == result )
		{
		numread = count;
		result = DTSNetUReceive( netUDP, srcaddr, queue->qData, &numread );
		if ( kNetworkNoData == result )
			{
			result = kNetworkPartialSend;
			}
		}
	if ( noErr == result )
		{
		if ( numread != count )
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
		result = FindInHash( srcaddr, &channel );
		}
	if ( noErr == result )
		{
		queue->qSize      = count;
		queue->qNext      = channel->netQueue;
		channel->netQueue = queue;
		}
	
	if ( noErr != result )
		{
		delete[] (char *) queue;
		}
	
	return result;
}


/*
**	DTSNetChannelPriv::ReadNetQueue()
**
**	return the first udp packet in the queue
*/
short
DTSNetChannelPriv::ReadNetQueue( void * data, short * psize )
{
	// bail if there is nothing in the queue
	DTSQueue * queue = netQueue;
	if ( nil == queue )
		{
		*psize = 0;
		return kNetworkNoData;
		}
	
	// copy the buffer
	long maxtocopy = *psize;
	long qsize     = queue->qSize;
	if ( maxtocopy > qsize )
		{
		maxtocopy = qsize;
		}
	memcpy( data, queue->qData, maxtocopy );
	*psize = maxtocopy;
	
	// free up the queue
	netQueue = queue->qNext;
	delete[] (char *) queue;
	
	return noErr;
}


/*
**	DTSNetChannelPriv::AddToHash()
**
**	add the channel to the hash table
*/
short
DTSNetChannelPriv::AddToHash( DTSNetChannelPriv * channel )
{
	long hash = channel->HashFunction();
	long size = 1 << netHostMax;
	DTSHostHash * start = netHostHash;
	DTSHostHash * table = start + hash;
	
	for ( long index = size;  index > 0;  --index )
		{
		if ( hash++ == size )
			{
			table = start;
			}
		
		DTSNetChannelPriv * test = table->hhChannel;
		if ( nil == test )
			{
			memcpy( table->hhAddr, &channel->netDstIPAddr[2], sizeof(table->hhAddr) );
			table->hhChannel = channel;
			return noErr;
			}
		
		++table;
		}
	
	return -1;
}


/*
**	DTSNetChannelPriv::RemoveFromHash()
**
**	remove the channel from the hash table
*/
void
DTSNetChannelPriv::RemoveFromHash()
{
	DTSNetChannelPriv * host = netHostChannel;
	if ( nil == host )
		{
		return;
		}
	
	long hash = HashFunction();
	long size = 1 << host->netHostMax;
	DTSHostHash * start = host->netHostHash;
	DTSHostHash * table = start + hash;
	
	for ( long index = size;  index > 0;  --index )
		{
		if ( hash++ == size )
			{
			table = start;
			}
		
		DTSNetChannelPriv * test = table->hhChannel;
		if ( this == test )
			{
			table->hhChannel = nil;
			return;
			}
		
		++table;
		}
}


/*
**	DTSNetChannelPriv::FindInHash()
**
**	return the channel with the ip address
*/
short
DTSNetChannelPriv::FindInHash( uchar * srcaddr, DTSNetChannelPriv ** pchannel )
{
	long shift = netHostMax;
	long hash = PrivHashFunction( srcaddr, shift );
	long size = 1 << shift;
	DTSHostHash * start = netHostHash;
	DTSHostHash * table = start + hash;
	DTSNetChannelPriv * channel = nil;
	long result = -1;
	
	for ( long index = size;  index > 0;  --index )
		{
		if ( hash++ == size )
			{
			table = start;
			}
		
		if ( 0 == memcmp( table->hhAddr, &srcaddr[2], sizeof(table->hhAddr) ) )
			{
			channel = table->hhChannel;
			if ( channel )
				{
				result = noErr;
				}
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
**	hash up the ip address to get an index into the table
*/
long
DTSNetChannelPriv::HashFunction()
{
	DTSNetChannelPriv * host = netHostChannel;
	if ( nil == host )
		{
		return 0;
		}
	
	long hash = PrivHashFunction( netDstIPAddr, host->netHostMax );
	
	return hash;
}


/*
**	PrivHashFunction()
**
**	hash up the ip address to get an index into the table
*/
static long
PrivHashFunction( uchar * addr, short shift )
{
	uchar lo = addr[2];
	uchar hi = addr[3];
	ulong hash = ( hi << 8 ) + lo;
	
	lo = addr[4];
	hi = addr[5];
	hash += ( hi << 8 ) + lo;
	
	lo = addr[6];
	hi = addr[7];
	hash += ( hi << 8 ) + lo;
	
	long mask = ( 1 << shift ) - 1;
	
	while ( hash & ~mask )
		{
		hash = ( hash & mask ) + ( hash >> shift );
		}
	
	return hash;
}


/*
**	DTSNetChannelPriv::FreeQueued()
**
**	free all elements in the queue linked list
*/
void
DTSNetChannelPriv::FreeQueued()
{
	DTSQueue * next;
	for ( DTSQueue * queue = netQueue;  queue;  queue = next )
		{
		next = queue->qNext;
		delete[] (char *) queue;
		}
	netQueue = nil;
}


/************************************************************************************/
#ifdef DTS_Mac


static	DTSBoolean		gOTInited;
static	InetSvcRef		gInetSvcRef;

static	pascal	void	MyNotifier( void * context, OTEventCode code, OTResult result, void * cookie );


/*
**	DTSInitNetwork()		+++	OPEN TRANSPORT +++
**
**	initialize the network
*/
short
DTSInitNetwork()
{
	// pick a random number for the unique identifier
	long id = GetRandom( 256 );
	id += id << 8;
	id += id << 16;
	gUniqueID = id;
	
	// initalize open transport
	long result = noErr;
	// if ( noErr == result )
		{
		result = InitOpenTransport();
		}
	if ( noErr == result )
		{
		gOTInited = true;
		if ( gInetSvcRef == 0 )
			{
			gInetSvcRef = OTOpenInternetServices( kDefaultInternetServicesPath, 0, &result );
			}
		}
	if ( noErr == result )
		{
		gNetworking = true;
		}
	
	return result;
}


/*
**	DTSExitNetwork()		+++	OPEN TRANSPORT +++
**
**	shutdown the network
*/
void
DTSExitNetwork()
{
	if ( gInetSvcRef )
		{
		OTCloseProvider( gInetSvcRef );
		gInetSvcRef = nil;
		}
	if ( gOTInited )
		{
		gOTInited = false;
		CloseOpenTransport();
		}
	
	gNetworking = false;
}


/*
**	DTSNetOpenTCP()		+++	OPEN TRANSPORT +++
**
**	open and bind a tcp end point
*/
static short
DTSNetOpenTCP( DTSEndPoint * pep, short * pport )
{
	short result = DTSNetOpen( pep, pport, kTCPName );
	return result;
}


/*
**	DTSNetOpenUDP()		+++	OPEN TRANSPORT +++
**
**	open and bind a udp end point
*/
static short
DTSNetOpenUDP( DTSEndPoint * pep, short * pport )
{
	short result = DTSNetOpen( pep, pport, kUDPName );
	return result;
}


/*
**	DTSNetOpen()		+++	OPEN TRANSPORT +++
**
**	open and bind a tcp or udp end point
*/
static short
DTSNetOpen( DTSEndPoint * pep, short * pport, char * path )
{
	InetAddress reqaddr;
	TBind reqbind;
	InetAddress retaddr;
	TBind retbind;
	
	DTSEndPoint ep = kClosedEndPoint;
	short reqport = 0;
	short retport = 0;
	if ( pport )
		{
		reqport = *pport;
		}
	short qlen = 0;
	if ( reqport )
		{
		qlen = 1;
		}
	
	OSStatus result = noErr;
	// if ( noErr == result )
		{
		OTConfiguration * config = OTCreateConfiguration( path );
		ep = OTOpenEndpoint( config, 0, nil, &result );
		}
	if ( noErr == result )
		{
		OTInitInetAddress( &reqaddr, reqport, 0 );
		reqbind.addr.buf    = (uchar *) &reqaddr;
		reqbind.addr.len    = sizeof(reqaddr);
		reqbind.addr.maxlen = sizeof(reqaddr);
		reqbind.qlen        = qlen;
		
		OTInitInetAddress( &retaddr, 0, 0 );
		retbind.addr.buf    = (uchar *) &retaddr;
		retbind.addr.len    = sizeof(retaddr);
		retbind.addr.maxlen = sizeof(retaddr);
		retbind.qlen        = 0;
		
		result = OTBind( ep, &reqbind, &retbind );
		}
	if ( noErr == result )
		{
		retport = retaddr.fPort;
		if ( 0       != reqport
		&&   retport != reqport )
			{
			result = kNetworkPortNotAvailable;
			}
		}
	
	if ( noErr != result )
		{
		DTSNetClose( ep );
		ep = kClosedEndPoint;
		retport = 0;
		}
	
	*pep = ep;
	if ( pport )
		{
		*pport = retport;
		}
	return result;
}


/*
**	DTSNetClose()		+++	OPEN TRANSPORT +++
**
**	close an endpoint
*/
static void
DTSNetClose( DTSEndPoint ep )
{
	if ( IsEndPointOpen(ep) )
		{
		OTCloseProvider( ep );
		}
}


/*
**	DTSNetAlloc()		+++	OPEN TRANSPORT +++
**
**	allocate a listen call structure
*/
static short
DTSNetAlloc( DTSEndPoint ep, DTSNetCall ** pcall )
{
	DTSNetCall * call = nil;
	
	OSStatus result = noErr;
	// if ( noErr == result )
		{
		call = (DTSNetCall *) OTAlloc( ep, T_CALL, T_ALL, &result );
		}
	
	*pcall = call;
	return result;
}


/*
**	DTSNetFree()		+++	OPEN TRANSPORT +++
**
**	free a listen call structure
*/
static void
DTSNetFree( DTSNetCall * call )
{
	if ( call )
		{
		OTFree( call, T_CALL );
		}
}


/*
**	DTSNetListen()		+++	OPEN TRANSPORT +++
**
**	listen for new connections
*/
static short
DTSNetListen( DTSEndPoint ep, DTSNetCall * call )
{
	// we have no conditions
	call->addr.len  = 0;
	call->opt.len   = 0;
	call->udata.len = 0;
	call->sequence  = 0;
	
	short result = noErr;
	// if ( noErr == result )
		{
		result = OTListen( ep, call );
		}
	
	// convert no data error
	if ( kOTNoDataErr == result )
		{
		result = 1;
		}
	
	return result;
}


/*
**	DTSNetAccept()		+++	OPEN TRANSPORT +++
**
**	accept new connections
*/
static short
DTSNetAccept( DTSEndPoint hostep, DTSEndPoint chanep, DTSNetCall * call )
{
	short result = noErr;
	// if ( noErr == result )
		{
		result = OTAccept( hostep, chanep, call );
		}
	
	// handle look errors
	if ( kOTLookErr == result )
		{
		result = OTLook( hostep );
		switch ( result )
			{
			// another listen event
			// will be handled by caller
			case T_LISTEN:
				result = kNetworkListen;
				break;
			
			// we've been disconnected
			case T_DISCONNECT:
				OTRcvDisconnect( hostep, nil );
				result = kNetworkDisconnect;
				break;
			
			// some other look event
			default:
				result = kNetworkUnknownLookErr;
				break;
			}
		}
	
	return result;
}


/*
**	DTSNetSend()		+++	OPEN TRANSPORT +++
**
**	send data
*/
static short
DTSNetSend( DTSEndPoint ep, void * data, short size )
{
	// send the data
	OTResult result = OTSnd( ep, data, size, 0 );
	
	// good result
	if ( size == result )
		{
		result = noErr;
		}
	// partial send error
	else
	if ( 0 <= result )
		{
		result = kNetworkPartialSend;
		}
	// look error
	else
	if ( kOTLookErr == result )
		{
		result = OTLook( ep );
		switch ( result )
			{
			// we've been disconnected
			case T_DISCONNECT:
				OTRcvDisconnect( ep, nil );
				result = kNetworkDisconnect;
				break;
			
			// we've been disconnected
			case T_ORDREL:
				OTRcvOrderlyDisconnect( ep );
				result = kNetworkDisconnect;
				break;
			
			// some other look event
			default:
				result = kNetworkUnknownLookErr;
				break;
			}
		}
	
	return result;
}


/*
**	DTSNetReceive()		+++	OPEN TRANSPORT +++
**
**	receive data
*/
static short
DTSNetReceive( DTSEndPoint ep, void * data, short * size )
{
	short numread = 0;
	OTFlags flags;
	OTResult result = OTRcv( ep, data, *size, &flags );
	
	// a good result
	if ( 0 <= result )
		{
		numread = result;
		result  = noErr;
		}
	// no data error
	else
	if ( kOTNoDataErr == result )
		{
		result = kNetworkNoData;
		}
	// handle look errors
	else
	if ( kOTLookErr == result )
		{
		result = OTLook( ep );
		switch ( result )
			{
			// we've been disconnected
			case T_DISCONNECT:
				OTRcvDisconnect( ep, nil );
				result = kNetworkDisconnect;
				break;
			
			// we've been disconnected
			case T_ORDREL:
				OTRcvOrderlyDisconnect( ep );
				result = kNetworkDisconnect;
				break;
			
			// some other look event
			default:
				result = kNetworkUnknownLookErr;
				break;
			}
		}
	
	// return the number read and the result
	*size = numread;
	return result;
}


/*
**	DTSNetUSend()		+++	OPEN TRANSPORT +++
**
**	send udp data
*/
static short
DTSNetUSend( DTSEndPoint ep, uchar * toaddr, void * data, short size )
{
	// initialize the structure
	TUnitData ud;
	ud.addr.maxlen  = 16;
	ud.addr.len     = 16;
	ud.addr.buf     = toaddr;
	ud.opt.maxlen   = 0;
	ud.opt.len      = 0;
	ud.opt.buf      = nil;
	ud.udata.maxlen = size;
	ud.udata.len    = size;
	ud.udata.buf    = (uchar *) data;
	
	// send the data
	OSStatus result = OTSndUData( ep, &ud );
	
	// handle look errors
	if ( kOTLookErr == result )
		{
		result = OTLook( ep );
		switch ( result )
			{
			// unit data error
			case T_UDERR:
				OTRcvUDErr( ep, nil );
				result = kNetworkUnitData;
				break;
			
			// some other look event
			default:
				result = kNetworkUnknownLookErr;
				break;
			}
		}
	
	return result;
}


/*
**	DTSNetUReceive()		+++	OPEN TRANSPORT +++
**
**	send udp data
*/
static short
DTSNetUReceive( DTSEndPoint ep, uchar * wantaddr, void * data, short * size )
{
	// separate the address we get from t_rcvudata
	// and the address the caller wants to receive data from
	uchar fromaddr[16];
	memcpy( fromaddr, wantaddr, sizeof(fromaddr) );
	
	// initialize the structure
	TUnitData ud;
	uchar optbuf[600];
	ud.addr.maxlen  = sizeof(fromaddr);
	ud.addr.len     = sizeof(fromaddr);
	ud.addr.buf     = fromaddr;
	ud.opt.maxlen   = sizeof(optbuf);
	ud.opt.len      = 0;
	ud.opt.buf      = optbuf;
	short numread   = *size;
	ud.udata.maxlen = numread;
	ud.udata.len    = numread;
	ud.udata.buf    = (uchar *) data;
	
	// read data
	OTFlags flags;
	numread = 0;
	OSStatus result = OTRcvUData( ep, &ud, &flags );
	
	// a good receive result
	if ( noErr == result )
		{
		// make sure we got the data from where we wanted to
		// a good result
		if ( wantaddr[1] != 2
		||   memcmp( fromaddr, wantaddr, sizeof(fromaddr) ) == 0 )
			{
			numread = ud.udata.len;
			
			// save the address if we need to
			if ( wantaddr[1] == 0 )
				{
				memcpy( wantaddr, fromaddr, sizeof(fromaddr) );
				}
			}
		}
	// no data error
	else
	if ( kOTNoDataErr == result )
		{
		result = kNetworkNoData;
		}
	// handle look errors
	else
	if ( kOTLookErr == result )
		{
		result = OTLook( ep );
		switch ( result )
			{
			// unit data error
			case T_UDERR:
				OTRcvUDErr( ep, nil );
				result = kNetworkUnitData;
				break;
			
			// some other look event
			default:
				result = kNetworkUnknownLookErr;
				break;
			}
		}
	
	// return the number read and the result
	*size = numread;
	return result;
}


/*
**	MyNotifier()
**
**	watch for an error or a connection
*/
static pascal void
MyNotifier( void * context, OTEventCode code, OTResult result, void * cookie )
{
	#pragma unused( cookie )
	
	long * myresult = (long *) context;
	
	if ( result < noErr )
		{
		*myresult = result;
		}
	
	switch ( code )
		{
		case T_CONNECT:
			if ( result >= 0 )
				{
				*myresult = 0;
				}
			break;
		
		case T_DISCONNECT:
			if ( result >= 0 )
				{
				*myresult = kNetworkDisconnect;
				}
			break;
		
		default:
			break;
		}
}


#if TARGET_API_MAC_CARBON
StaticUPP(MyNotifier,OTNotify)
#endif


/*
**	DTSNetConnect()		+++	OPEN TRANSPORT +++
**
**	connect to a host
*/
static short
DTSNetConnect( DTSEndPoint ep, uchar * hostaddr, DTSNetChannelPriv * channel )
{
	DTSNetCall reqcall;
	volatile long notifierresult;
	DTSBoolean bNotifierInstalled = false;
	DTSBoolean bSetAsynchronous = false;
	
	short result = noErr;
	// if ( noErr == result )
		{
		notifierresult = 1;
#if TARGET_API_MAC_CARBON
		result = OTInstallNotifier( ep, MakeUPP(MyNotifier,OTNotify), (void *) &notifierresult );
#else
		result = OTInstallNotifier( ep, MyNotifier, (void *) &notifierresult );
#endif
		}
	if ( noErr == result )
		{
		bNotifierInstalled = true;
		result = OTSetAsynchronous( ep );
		}
	if ( noErr == result )
		{
		bSetAsynchronous = true;
		
		reqcall.addr.maxlen  = 16;
		reqcall.addr.len     = 16;
		reqcall.addr.buf     = hostaddr;
		reqcall.opt.maxlen   = 0;
		reqcall.opt.len      = 0;
		reqcall.opt.buf      = nil;
		reqcall.udata.maxlen = 0;
		reqcall.udata.len    = 0;
		reqcall.udata.buf    = nil;
		
		result = OTConnect( ep, &reqcall, nil );
		if ( kOTNoDataErr == result )
			{
			result = noErr;
			}
		}
	if ( noErr == result )
		{
		for(;;)
			{
			result = notifierresult;
			if ( noErr >= result )
				{
				break;
				}
			result = channel->ConnectCallBack( "Waiting for host." );
			if ( noErr != result )
				{
				break;
				}
			}
		}
	
	if ( bSetAsynchronous )
		{
		result = OTSetSynchronous( ep );
		}
	if ( bNotifierInstalled )
		{
		OTRemoveNotifier( ep );
		}
	
	if ( noErr == result )
		{
		result = OTRcvConnect( ep, nil );
		}
	if ( kNetworkDisconnect == result )
		{
		OTRcvDisconnect( ep, nil );
		}
	
	// handle look errors
	if ( kOTLookErr == result )
		{
		result = OTLook( ep );
		switch ( result )
			{
			// we've been disconnected
			case T_DISCONNECT:
				OTRcvDisconnect( ep, nil );
				result = kNetworkDisconnect;
				break;
			
			// we've been disconnected
			case T_ORDREL:
				OTRcvOrderlyDisconnect( ep );
				result = kNetworkDisconnect;
				break;
			
			// some other look event
			default:
				result = kNetworkUnknownLookErr;
				break;
			}
		}
	
	return result;
}


/*
**	DTSNetResolveName()		+++	OPEN TRANSPORT +++
**
**	resolve the domain name to an ip address
*/
static short
DTSNetResolveName( DTSEndPoint ep, char * str, uchar * addr )
{
	DNSAddress dnsAddr;
	TBind reqAddr;
	reqAddr.addr.buf    = (uchar *) &dnsAddr;
	reqAddr.addr.len    = OTInitDNSAddress( &dnsAddr, str );
	reqAddr.addr.maxlen = sizeof(dnsAddr);
	reqAddr.qlen        = 0;
	
	TBind retAddr;
	retAddr.addr.buf    = addr;
	retAddr.addr.len    = 16;
	retAddr.addr.maxlen = 16;
	retAddr.qlen        = 0;
	
	// to be really fancy, we'd do this asynchronously too...
	OSStatus result = OTResolveAddress( ep, &reqAddr, &retAddr, TIMEOUT );
		
	return result;
}


/*
**	Random junk for getting the ethernet hardware address
*/
enum {
	kUnsupported 		= 0,
	kPDMMachine			= 1,
	kPCIMachine			= 2,
	kCommSlotMachine	= 3,
	kPCIComm2Machine 	= 4
};

enum {
	kPDMEnetROMBase	= 0x50f08000
};

static	UInt32			DoesCPUHaveBuiltInEthernet(void);
static	OSStatus		GetPDMBuiltInEnetAddr(UInt8 *enetaddr);
static	UInt8			ByteSwapValue(UInt8 val);
static	OSStatus		GetPCIBuiltInEnetAddr(UInt8 *enetaddr);
static	OSStatus		GetPCIComm2EnetAddr(UInt8 *enetaddr);


/*
**	DTSEthernetHardwareAddress()
**
**	return the 6 byte ethernet hardware address
*/
long
DTSEthernetHardwareAddress( void * buff )
{
	// first, attempt to get the burned-in ethernet address 
	// directly from ROM or NameRegistry

#if !TARGET_API_MAC_CARBON
	UInt8 enetaddr[6] = {0};
	UInt32 cputype = DoesCPUHaveBuiltInEthernet();
	OSStatus err;
	
	switch (cputype)
		{
		case kPDMMachine:
			err = GetPDMBuiltInEnetAddr(enetaddr);
			if (noErr == err)
				{
				memcpy( buff, enetaddr, sizeof(enetaddr) );
				return noErr;
				}
			break;
		
		case kPCIMachine:
			err = GetPCIBuiltInEnetAddr(enetaddr);
			if (noErr == err)
				{
				memcpy( buff, enetaddr, sizeof(enetaddr) );
				return noErr;
				}
			break;
		
		case kCommSlotMachine:
			// This is a NuBus based Power Mac which may have a CommSlot Ethernet card
			// must use Open Transport to obtain the ethernet address
			break;
		
		case kPCIComm2Machine:
			// This is a PCI system which may have a CommSlot2 Ethernet card
			// must use Open Transport to obtain the ethernet address
			
			// or not?
			err = GetPCIComm2EnetAddr(enetaddr);
			if (noErr == err)
				{
				memcpy( buff, enetaddr, sizeof(enetaddr) );
				return noErr;
				}
			break;
		
		default:
			// It appears that this CPU does not have built-in ethernet
			break;
		}
#endif		// ! TARGET_API_MAC_CARBON
	
	// that didn't work
	// ask open transport
	struct Address8022
	{
		OTAddressType	fAddrFamily;
		UInt8			fHWAddr[k48BitAddrLength];
		UInt16			fSAP;
		UInt8			fSNAP[k8022SNAPLength];
	};
	typedef struct Address8022 Address8022;
	
	Address8022 theAddr       = {AF_8022, {0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 0x8888, {0x00,0x00,0x00,0x00,0x00}};
	Address8022 theReturnAddr = {AF_8022, {0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 0x8888, {0x00,0x00,0x00,0x00,0x00}};
	
	for ( long index = 0;  ;  ++index )
		{
		OTPortRecord devicePortRecord;
		DTSBoolean foundAPort = OTGetIndexedPort( &devicePortRecord, index );
		if ( foundAPort == false )
			return -1;
		
		if ( ( devicePortRecord.fCapabilities & kOTPortIsDLPI ) == 0 )
			continue;
		if ( ( devicePortRecord.fCapabilities & kOTPortIsTPI ) == 0 )
			continue;
		
		long deviceType = OTGetDeviceTypeFromPortRef( devicePortRecord.fRef );
		if ( deviceType != kOTEthernetDevice )
			continue;
		
		OTConfiguration * config = OTCreateConfiguration( devicePortRecord.fPortName );
		if ( config == nil )
			continue;
		
		OSStatus result; 
		EndpointRef ep = OTOpenEndpoint( config, (OTOpenFlags) 0, nil, &result );
		if ( result != noErr )
			continue;
		
		TBind requestInfo;
		requestInfo.addr.buf    = (UInt8 *) &theAddr;
		requestInfo.addr.len    = 10;
		requestInfo.addr.maxlen = 0;			
		requestInfo.qlen        = 0;
		result = OTBind( ep, &requestInfo, nil );
		
		if ( noErr == result )
			{
			TBind returnInfo;
			returnInfo.addr.buf    = (UInt8 *) &theReturnAddr;
			returnInfo.addr.maxlen = 10;
			returnInfo.qlen        = 0;
			
			result = OTGetProtAddress( ep, &returnInfo, nil );
			
			OTUnbind( ep );
			}
		
		OTCloseProvider( ep );
		
		if ( result != noErr )
			continue;
		
		memcpy( buff, theReturnAddr.fHWAddr, 6 );
		return noErr;
		}
}

#if !TARGET_API_MAC_CARBON

/*
**	DoesCPUHaveBuiltInEthernet()
**
**	stolen from that other dts
*/
static UInt32
DoesCPUHaveBuiltInEthernet(void)
{
	long		response = kUnsupported;
	OSStatus	err;
	ulong		result = kUnsupported;
	
	err = Gestalt(gestaltMachineType, &response);
	switch (response)
		{
		case gestaltPowerMac8100_120:
		case gestaltAWS9150_80:
		case gestaltPowerMac8100_110:
		case gestaltPowerMac7100_80:
		case gestaltPowerMac8100_100:
		case gestaltAWS9150_120:
		case gestaltPowerMac8100_80:
		case gestaltPowerMac6100_60:
		case gestaltPowerMac6100_66:
		case gestaltPowerMac7100_66:
			result = kPDMMachine;
			break;
		
		case gestaltPowerMac9500:
		case gestaltPowerMac7500:
		case gestaltPowerMac8500:
		case gestaltPowerBook3400:
		case gestaltPowerBookG3:
		case gestaltPowerMac7200:
		case gestaltPowerMac7300:
		case gestaltPowerBookG3Series:
		case gestaltPowerBookG3Series2:
		case gestaltPowerMacG3:
		case gestaltPowerMacNewWorld:
			result = kPCIMachine;
			break;
			
		case gestaltPowerMac5200:
		case gestaltPowerMac6200:
			result = kCommSlotMachine;
			break;
			
		case gestaltPowerMac6400:
		case gestaltPowerMac5400:
		case gestaltPowerMac5500:
		case gestaltPowerMac6500:
		case gestaltPowerMac4400_160:
		case gestaltPowerMac4400:
			result = kPCIComm2Machine;
			break;
		}
	
	if (response == kUnsupported)
		{
		err = Gestalt(gestaltNameRegistryVersion, &response);
		if (noErr == err)
			result = kPCIMachine;
		}
	
	return result;
}


/*
**	GetPDMBuiltInEnetAddr()
**
**	from that other dts
*/
static OSStatus
GetPDMBuiltInEnetAddr(UInt8 *enetaddr)
{
	for (UInt32 i = 0; i < 6; i++)
	{
		UInt8 * val = (UInt8 *)(kPDMEnetROMBase + i * 0x10);
		enetaddr[i] = ByteSwapValue(*val);
	}
	return noErr;
}


/*
**	ByteSwapValue()
**
**	from that other dts
*/
static UInt8
ByteSwapValue(UInt8 val)
{
	UInt8	result = 0;
	
	if (val & 0x01)
		result |= 0x80;
		
	if (val & 0x02)
		result |= 0x40;
		
	if (val & 0x04)
		result |= 0x20;
		
	if (val & 0x08)
		result |= 0x10;
		
	if (val & 0x10)
		result |= 0x08;
		
	if (val & 0x20)
		result |= 0x04;
		
	if (val & 0x40)
		result |= 0x02;
		
	if (val & 0x80)
		result |= 0x01;
	return result;
}


/*
**	GetPCIBuiltInEnetAddr()
**
**	from that other dts
*/
static OSStatus
GetPCIBuiltInEnetAddr(UInt8 *enetaddr)
{
	// hey, it could happen
#if __MC68K__
	#pragma unused (enetaddr)
	return -1;
#else
	OSStatus				err = noErr;
    RegEntryIter            cookie;
    RegEntryID              theFoundEntry;
    char		          	enetAddrStr[32] = "local_mac-address";
    RegCStrEntryNamePtr     enetAddrCStr = enetAddrStr;
    RegEntryIterationOp     iterOp;
    UInt8					enetAddr[6];
    DTSBoolean              done = false;
	
	long data;
	err = Gestalt( gestaltNameRegistryVersion, &data );
	if ( err != noErr )
		return err;
	
    err = RegistryEntryIDInit( &theFoundEntry );
	if (err != noErr)
		return err;

    err = RegistryEntryIterateCreate( &cookie );
	if (err != noErr)
		return err;

    iterOp = kRegIterDescendants;

    err = RegistryEntrySearch( &cookie, iterOp, &theFoundEntry, &done,
                                                       enetAddrCStr, nil, 0);
    
    if (noErr == err)
    {
	    RegPropertyValueSize theSize = sizeof(enetAddr);
	    err = RegistryPropertyGet(&theFoundEntry, enetAddrCStr, &enetAddr, &theSize );
	    if (noErr == err)
	    	BlockMoveData(enetAddr, enetaddr, sizeof(enetAddr));
	}

	RegistryEntryIterateDispose( &cookie );

	return err;
#endif	/* ! 68K */
}


/*
**	GetPCIComm2EnetAddr()
**
**	from that other dts
*/
static OSStatus
GetPCIComm2EnetAddr(UInt8 *enetaddr)
{
	// hey, it could happen
#if __MC68K__
	#pragma unused (enetaddr)
	return -1;
#else
	OSStatus				err = noErr;
    RegEntryIter            cookie;
    RegEntryID              theFoundEntry;
    char		          	enetAddrStr[32] = "ASNT,ethernet-address";
    RegCStrEntryNamePtr     enetAddrCStr = enetAddrStr;
    RegEntryIterationOp     iterOp;
    UInt8					*enetAddr;
    DTSBoolean              done = false;
	
	long data;
	err = Gestalt( gestaltNameRegistryVersion, &data );
	if ( err != noErr )
		return err;
	
    err = RegistryEntryIDInit( &theFoundEntry );
	if ( err != noErr )
	{
//		fprintf(stdout, "RegistryEntryIDInit failed\n");
		return err;
	}

    err = RegistryEntryIterateCreate( &cookie );
	if ( err != noErr )
	{
//		fprintf(stdout, "RegistryEntryIterateCreate failed\n");
		return err;
	}

    iterOp = kRegIterDescendants;

    err = RegistryEntrySearch( &cookie, iterOp, &theFoundEntry, &done,
                                                       enetAddrCStr, nil, 0);
    
    if ( noErr == err )
		{
		RegPropertyValueSize theSize = sizeof(enetAddr);
		err = RegistryPropertyGet(&theFoundEntry, enetAddrCStr, &enetAddr, &theSize );
		if ( noErr == err )
			BlockMoveData(enetAddr, enetaddr, 6);
		}

    RegistryEntryIterateDispose( &cookie );

	return err;
#endif	// 68K
}

#endif	// ! TARGET_API_MAC_CARBON

#endif
/************************************************************************************/
#ifdef DTS_Unix

// additional includes
#include <tiuser.h>

// define these if they are missing
#ifndef O_RDWR
#define		O_RDWR			2
#endif
#ifndef O_NDELAY
#define		O_NDELAY		4
#endif


/*
**	DTSInitNetwork()		+++ UNIX +++
**
**	initialize the network
*/
short
DTSInitNetwork()
{
	// pick a random number for the unique identifier
	long id = GetRandom( 256 );
	id += id << 8;
	id += id << 16;
	gUniqueID = id;
	
	return noErr;
}


/*
**	DTSExitNetwork()		+++ UNIX +++
**
**	shutdown the network
*/
void
DTSExitNetwork()
{
}


/*
**	DTSNetOpenTCP()		+++ UNIX +++
**
**	open and bind a tcp end point
*/
static short
DTSNetOpenTCP( DTSEndPoint * pep, short * pport )
{
	short result = DTSNetOpen( pep, pport, "/dev/tcp" );
	return result;
}


/*
**	DTSNetOpenUDP()		+++ UNIX +++
**
**	open and bind a udp end point
*/
static short
DTSNetOpenUDP( DTSEndPoint * pep, short * pport )
{
	short result = DTSNetOpen( pep, pport, "/dev/udp" );
	return result;
}


/*
**	DTSNetOpen()		+++ UNIX +++
**
**	open and bind a tcp or udp end point
*/
static short
DTSNetOpen( DTSEndPoint * pep, short * pport, char * path )
{
	uchar reqaddr[16];
	struct t_bind reqbind;
	uchar retaddr[16];
	struct t_bind retbind;
	
	DTSEndPoint ep = kClosedEndPoint;
	short reqport = 0;
	short retport = 0;
	if ( pport )
		{
		reqport = *pport;
		}
	short qlen = 0;
	if ( reqport )
		{
		qlen = 1;
		}
	
	long result = noErr;
	// if ( noErr == result )
		{
		ep = t_open( path, O_RDWR + O_NDELAY, nil );
		
		if ( ep < 0 )
			{
			result = - t_errno;
			}
		}
	if ( noErr == result )
		{
		memset( reqaddr, 0, sizeof(reqaddr) );
		reqaddr[1] = 2;
		reqaddr[2] = reqport >> 8;
		reqaddr[3] = reqport;
		
		reqbind.addr.buf    = (char *)  reqaddr;
		reqbind.addr.len    = sizeof(reqaddr);
		reqbind.addr.maxlen = sizeof(reqaddr);
		reqbind.qlen        = qlen;
		
		memset( retaddr, 0, sizeof(retaddr) );
		retaddr[1] = 2;
		
		retbind.addr.buf    = (char *) retaddr;
		retbind.addr.len    = sizeof(retaddr);
		retbind.addr.maxlen = sizeof(retaddr);
		retbind.qlen        = 0;
		
		result = t_bind( ep, &reqbind, &retbind );
		}
	if ( noErr == result )
		{
		uchar porthi = retaddr[2];
		uchar portlo = retaddr[3];
		retport = ( porthi << 8 ) + portlo;
		
		if ( 0       != reqport
		&&   retport != reqport )
			{
			result = kNetworkPortNotAvailable;
			}
		}
	
	if ( noErr != result )
		{
		DTSNetClose( ep );
		ep = kClosedEndPoint;
		retport = 0;
		}
	
	*pep = ep;
	if ( pport )
		{
		*pport = retport;
		}
	return result;
}


/*
**	DTSNetClose()		+++ UNIX +++
**
**	close an endpoint
*/
static void
DTSNetClose( DTSEndPoint ep )
{
	if ( IsEndPointOpen(ep) )
		{
		t_close( ep );
		}
}


/*
**	DTSNetAlloc()		+++ UNIX +++
**
**	allocate a listen call structure
*/
static short
DTSNetAlloc( DTSEndPoint ep, DTSNetCall ** pcall )
{
	DTSNetCall * call = nil;
	
	long result = noErr;
	// if ( noErr == result )
		{
		call = (DTSNetCall *) t_alloc( ep, T_CALL, T_ALL );
		if ( call == nil )
			{
			result = memFullErr;
			}
		}
	
	*pcall = call;
	return result;
}


/*
**	DTSNetFree()		+++ UNIX +++
**
**	free a listen call structure
*/
static void
DTSNetFree( DTSNetCall * call )
{
	if ( call )
		{
		t_free( (char *) call, T_CALL );
		}
}


/*
**	DTSNetListen()		+++ UNIX +++
**
**	listen for new connections
*/
static short
DTSNetListen( DTSEndPoint ep, DTSNetCall * call )
{
	// we have no conditions
	call->addr.len  = 0;
	call->opt.len   = 0;
	call->udata.len = 0;
	call->sequence  = 0;
	
	int result = t_listen( ep, call );
	
	// check errors
	if ( 0 != result )
		{
		result = - t_errno;
		}
	
	// convert no data error
	if ( -TNODATA == result )
		{
		result = 1;
		}
	
	return result;
}


/*
**	DTSNetAccept()		+++ UNIX +++
**
**	accept new connections
*/
static short
DTSNetAccept( DTSEndPoint hostep, DTSEndPoint chanep, DTSNetCall * call )
{
	int result = t_accept( hostep, chanep, call );
	
	// check errors
	if ( 0 != result )
		{
		result = - t_errno;
		}
	
	// handle look errors
	if ( -TLOOK == result )
		{
		result = t_look( hostep );
		switch ( result )
			{
			// another listen event
			// will be handled by caller
			case T_LISTEN:
				result = kNetworkListen;
				break;
			
			// we've been disconnected
			case T_DISCONNECT:
				t_rcvdis( hostep, nil );
				result = kNetworkDisconnect;
				break;
			
			// some other look event
			default:
				result = kNetworkUnknownLookErr;
				break;
			}
		}
	
	return result;
}


/*
**	DTSNetSend()		+++ UNIX +++
**
**	send data
*/
static short
DTSNetSend( DTSEndPoint ep, void * data, short size )
{
	// send the data
	int result = t_snd( ep, (char *) data, size, 0 );
	
	// good result
	if ( size == result )
		{
		result = noErr;
		}
	// partial send error
	else
	if ( 0 <= result )
		{
		result = kNetworkPartialSend;
		}
	// some other error
	else
		{
		result = - t_errno;
		}
	
	// handle look errors
	if ( -TLOOK == result )
		{
		result = t_look( ep );
		switch ( result )
			{
			// we've been disconnected
			case T_DISCONNECT:
				t_rcvdis( ep, nil );
				result = kNetworkDisconnect;
				break;
			
			// we've been disconnected
			case T_ORDREL:
				t_rcvrel( ep );
				result = kNetworkDisconnect;
				break;
			
			// some other look event
			default:
				result = kNetworkUnknownLookErr;
				break;
			}
		}
	
	return result;
}


/*
**	DTSNetReceive()		+++ UNIX +++
**
**	receive data
*/
static short
DTSNetReceive( DTSEndPoint ep, void * data, short * size )
{
	short numread = 0;
	int flags;
	int result = t_rcv( ep, (char *) data, *size, &flags );
	
	// a good result
	if ( 0 <= result )
		{
		numread = result;
		result  = noErr;
		}
	// an error result
	else
		{
		result = - t_errno;
		}
	// no data error
	if ( -TNODATA == result )
		{
		result = kNetworkNoData;
		}
	
	// handle look errors
	if ( -TLOOK == result )
		{
		result = t_look( ep );
		switch ( result )
			{
			// we've been disconnected
			case T_DISCONNECT:
				t_rcvdis( ep, nil );
				result = kNetworkDisconnect;
				break;
			
			// we've been disconnected
			case T_ORDREL:
				t_rcvrel( ep );
				result = kNetworkDisconnect;
				break;
			
			// some other look event
			default:
				result = kNetworkUnknownLookErr;
				break;
			}
		}
	
	// return the number read and the result
	*size = numread;
	return result;
}


/*
**	DTSNetUSend()		+++ UNIX +++
**
**	send udp data
*/
static short
DTSNetUSend( DTSEndPoint ep, uchar * toaddr, void * data, short size )
{
	// initialize the structure
	struct t_unitdata ud;
	ud.addr.maxlen  = 16;
	ud.addr.len     = 16;
	ud.addr.buf     = (char *) toaddr;
	ud.opt.maxlen   = 0;
	ud.opt.len      = 0;
	ud.opt.buf      = nil;
	ud.udata.maxlen = size;
	ud.udata.len    = size;
	ud.udata.buf    = (char *) data;
	
	// send the data
	int result = t_sndudata( ep, &ud );
	
	// handle errors
	if ( 0 > result )
		{
		result = - t_errno;
		}
	
	// handle look errors
	if ( -TLOOK == result )
		{
		result = t_look( ep );
		switch ( result )
			{
			// unit data error
			case T_UDERR:
				t_rcvuderr( ep, nil );
				result = kNetworkUnitData;
				break;
			
			// some other look event
			default:
				result = kNetworkUnknownLookErr;
				break;
			}
		}
	
	return result;
}


/*
**	DTSNetUReceive()		+++ UNIX +++
**
**	send udp data
*/
static short
DTSNetUReceive( DTSEndPoint ep, uchar * wantaddr, void * data, short * size )
{
	// separate the address we get from t_rcvudata
	// and the address the caller wants to receive data from
	uchar fromaddr[16];
	memcpy( fromaddr, wantaddr, sizeof(fromaddr) );
	
	// initialize the structure
	struct t_unitdata ud;
	uchar optbuf[600];
	ud.addr.maxlen  = sizeof(fromaddr);
	ud.addr.len     = sizeof(fromaddr);
	ud.addr.buf     = (char *) fromaddr;
	ud.opt.maxlen   = sizeof(optbuf);
	ud.opt.len      = 0;
	ud.opt.buf      = (char *) optbuf;
	short numread   = *size;
	ud.udata.maxlen = numread;
	ud.udata.len    = numread;
	ud.udata.buf    = (char *) data;
	
	// read data
	int flags;
	numread = 0;
	int result = t_rcvudata( ep, &ud, &flags );
	
	// a good receive result
	if ( 0 == result )
		{
		// make sure we got the data from where we wanted to
		// a good result
		if ( wantaddr[1] != 2
		||   memcmp( fromaddr, wantaddr, sizeof(fromaddr) ) == 0 )
			{
			numread = ud.udata.len;
			
			// save the address if we need to
			if ( wantaddr[1] == 0 )
				{
				memcpy( wantaddr, fromaddr, sizeof(fromaddr) );
				}
			}
		}
	// handle errors
	else
		{
		result = - t_errno;
		}
	
	// no data error
	if ( -TNODATA == result )
		{
		result = kNetworkNoData;
		}
	// handle look errors
	else
	if ( -TLOOK == result )
		{
		result = t_look( ep );
		switch ( result )
			{
			// unit data error
			case T_UDERR:
				t_rcvuderr( ep, nil );
				result = kNetworkUnitData;
				break;
			
			// some other look event
			default:
				result = kNetworkUnknownLookErr;
				break;
			}
		}
	
	// return the number read and the result
	*size = numread;
	return result;
}


/*
**	DTSNetConnect()		+++ UNIX +++
**
**	connect to a host
*/
static short
DTSNetConnect( DTSEndPoint ep, uchar * hostaddr, DTSNetChannelPriv * channel )
{
	channel = nil;
	DTSNetCall reqcall;
	
	short result = noErr;
	// if ( noErr == result )
		{
		reqcall.addr.maxlen  = 16;
		reqcall.addr.len     = 16;
		reqcall.addr.buf     = (char *) hostaddr;
		reqcall.opt.maxlen   = 0;
		reqcall.opt.len      = 0;
		reqcall.opt.buf      = nil;
		reqcall.udata.maxlen = 0;
		reqcall.udata.len    = 0;
		reqcall.udata.buf    = nil;
		
		result = t_connect( ep, &reqcall, nil );
		
		if ( 0 > result )
			{
			result = - t_errno;
			}
		}
	
	// handle look errors
	if ( -TLOOK == result )
		{
		result = t_look( ep );
		switch ( result )
			{
			// we've been disconnected
			case T_DISCONNECT:
				t_rcvdis( ep, nil );
				result = kNetworkDisconnect;
				break;
			
			// we've been disconnected
			case T_ORDREL:
				t_rcvrel( ep );
				result = kNetworkDisconnect;
				break;
			
			// some other look event
			default:
				result = kNetworkUnknownLookErr;
				break;
			}
		}
	
	return result;
}


/*
**	DTSNetResolveName()		+++	UNIX +++
**
**	not supported
**	resolve the domain name to an ip address
*/
static short
DTSNetResolveName( DTSEndPoint ep, char * str, uchar * addr )
{
#pragma unused( ep, str, addr )
	
	return -1;
}


/*
**	DTSEthernetHardwareAddress()		+++ UNIX +++
**
**	not supported
*/
long
DTSEthernetHardwareAddress( void * addr )
{
#pragma unused (addr)
	
	return -1;
}


/*
**	setnetfds()
**
**	hack for handing off end points
*/
void
setnetfds( int tcpfd, int udpfd )
{
	handofftcp = tcpfd;
	handoffudp = udpfd;
}


/*
**	getnetfds()
**
**	hack for handing off end points
*/
long
getnetfds( int * ptcpfd, int * pudpfd )
{
	if ( handofftcp == 0
	||   handoffudp == 0 )
		return -1;
	
	*ptcpfd = handofftcp;
	*pudpfd = handoffudp;
	
	return 0;
}


#endif
/************************************************************************************/
