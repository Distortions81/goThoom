/*
**	Network_dts.h		dtslib2
**
**	This document defines the interface for the DTSLib library.
**	The goal here is transportability.
**	The library must implement this interface exactly.
**	Any application built using this library is not allowed to use
**	anything that could be machine or implementation dependent.
**	That implies the following:
**		Do not use int. Always use char, short, or long.
**		Do not (permanently) use any libraries for which we don't have source.
**		Do not use try and recover because they are not always implemented.
**	All of the following classes can have private parts not shown here.
**	Do not pass any arguments to constructors.
**	Pass horizontal coords before vertical.
**	Pass by pointer not by reference.
**	Use C strings instead of pascal strings.
**
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

#ifndef Network_dts_h
#define Network_dts_h

#ifndef _dtslib2_
You must include Prefix_XXDebug_XXX.h.
#endif


/*
**	Network classes and routines
*/
short DTSInitNetwork( void );					// initialize networking
void DTSExitNetwork( void );					// close down networking
long DTSEthernetHardwareAddress( void * addr );	// get the 6 byte ethernet hardware address


/*
**	class DTSNetChannel
*/
class DTSNetChannelPriv;
class DTSNetChannel : public DTSImplement<DTSNetChannelPriv>
	{
public:
	// host listens on this port
	short InitHost( short port, short max );
	
	// host connects to a new client
	short AnswerClient( DTSNetChannel ** channel );
	
	// client block till we connect to the host
	short ConnectHost( char * hostaddr, short port=0 );
	
	// client callback so we can terminate early and show status messages
	virtual short ConnectCallBack( char * status );
	
	// read from the channel
	// size initially holds max to read
	short Read( void * data, short * size );
	
	// write to the channel
	// reliable=tcp unreliable=udp
	short Write( const void * data, short size, short type );
	
	// return non-zero if connected
	DTSBoolean IsConnected( void );
	
	// close all endpoints
	void Close( void );
	};


#ifdef DTS_Mac
typedef		EndpointRef		DTSEndPoint;
#else
typedef		int				DTSEndPoint;
#endif

class PartialPacket
	{
public:
	uchar *		ppBuffer;		// pointer to the buffer to hold the packet
	short		ppMaxSize;		// the size of the packet buffer
	short		ppSize;			// the size of the packet to be read
	short		ppRead;			// the number of bytes already read
	
	// constructor/destructor
	PartialPacket( void );
	~PartialPacket( void );
	
	// interface
	short Allocate( void );
	short ReadData( DTSEndPoint ep, uchar * unreliableAddr, void * buffer, short * size );
	short FillPartialBuffer( DTSEndPoint ep, uchar * unreliableAddr );
	void Reset( void );
	};

class DTSConnectList;
class DTSQueue;
class DTSHostHash;

class DTSNetChannel
	{
private:
	short				netPort;
	DTSBoolean			netConnected;
	DTSEndPoint			netTCP;
	DTSEndPoint			netUDP;
	DTSConnectList *	netConnectList;
	uchar				netDstIPAddr[16];
	PartialPacket		netTCPPacket;
	PartialPacket		netUDPPacket;
	DTSNetChannel *		netHostChannel;
	DTSQueue *			netQueue;
	DTSHostHash *		netHostHash;
	short				netHostMax;
	
public:
	// constructor/destructor
	DTSNetChannel( void );
	virtual ~DTSNetChannel( void );
	
	// host interface
	short InitHost( short port, short max );			// listen on this port
	short AnswerClient( DTSNetChannel ** channel );		// connect to a new client
	
	// client interface
	short ConnectHost( char * hostaddr, short port=0 );	// block till we connect to the host
	virtual short ConnectCallBack( char * status );		// callback so we can terminate early
														// and show status messages
	
	// shared interface
	short Read( void * data, short * size );				// size initially holds max to read
	short Write( const void * data, short size, short type );	// reliable=tcp unreliable=udp
	DTSBoolean IsConnected( void );							// return non-zero if connected
	void Close( void );										// close all endpoints
	
	// private interface
private:
	DTSConnectList * FindConnection( short state );
	short HandleConnectList( void );
	short CreateConnect( void );
	DTSBoolean Handle1Connect( DTSConnectList * connect );
	DTSBoolean DoListenTask( DTSConnectList * connect );
	DTSBoolean DoAcceptTask( DTSConnectList * connect );
	DTSBoolean DoSendUDPTask( DTSConnectList * connect );
	DTSBoolean DoWaitUDPTask( DTSConnectList * connect );
	DTSBoolean DoConfirmTask( DTSConnectList * connect );
	short ConnectTCP( void );
	short ConnectUDP( void );
	short ConnectUDPWait( long * id );
	short ConnectUDPExchange( long id );
	void FreeQueued( void );
	short HandleHostQueue( void );
	short HandleUDP( void );
	short Handle1UDP( void );
	short Handle1NewUDP( uchar * srcaddr );
	short Handle1ExistingUDP( uchar * srcaddr, short count );
	short ReadNetQueue( void * data, short * size );
	short AddToHash( DTSNetChannel * channel );
	void RemoveFromHash( void );
	short FindInHash( uchar * addr, DTSNetChannel ** channel );
	long HashFunction( void );
	};

enum
	{
	kNetWriteReliable,
	kNetWriteUnreliable
	};


#endif // Network_dts_h
