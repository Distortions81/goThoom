/*
**	Network_dts.h		dtslib2
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
#include "Prefix_dts.h"
#endif

#include <netinet/in.h>

#include "Local_dts.h"
#include "Platform_dts.h"


/*
**	Network classes and routines
*/

// class DTSNetAddress -- encapsulate an IP address
class DTSNetAddress
{
private:
	sockaddr_in		addr;	// one day, handle IPv6 too
	
public:
					DTSNetAddress();
					DTSNetAddress( const sockaddr * inAddr );
					~DTSNetAddress() {}
#if __cplusplus >= 201103
					DTSNetAddress( const DTSNetAddress& ) = default;
	DTSNetAddress&	operator=( const DTSNetAddress& ) = default;
#endif
	
	void			Set( const sockaddr * src );
	void			Set( sa_family_t fam, in_addr_t address, in_port_t = 0 );
	void			Reset();
	
	sockaddr * 		sa()			{ return reinterpret_cast<sockaddr*>( &addr ); }
	const sockaddr * sa() const		{ return reinterpret_cast<const sockaddr *>( &addr ); }
	socklen_t		sa_len() const	{ return sizeof addr; }
	
	bool			operator==( const DTSNetAddress& ) const;
	
	in_port_t		Port() const	{ return ntohs( addr.sin_port ); }
	sa_family_t		Family() const	{ return addr.sin_family; }
	in_addr_t		IPAddr() const	{ return ntohl( addr.sin_addr.s_addr ); }
	
	const char *	ToString( char * dst, size_t dstLen ) const;
};


/*
**	class DTSNetChannel
*/
class DTSNetChannelPriv;
class DTSNetChannel
{
public:
	DTSImplementNoCopy<DTSNetChannelPriv> priv;
	
						DTSNetChannel();
	virtual				~DTSNetChannel();
	
	// host listens on this port
	DTSError			InitHost( ushort port, uint maxConns );
	
	// host connects to a new client
	DTSError			AnswerClient( DTSNetChannel ** channel );
	
	// client block till we connect to the host
	DTSError			ConnectHost( const char * hostaddr, ushort port = 0 );
	
	// client callback so we can terminate early and show status messages
	virtual DTSError	ConnectCallBack( const char * status );
	
	// read from the channel
	// size initially holds max to read
	DTSError			Read( void * data, size_t * size );
	
	// write to the channel
	// reliable=tcp unreliable=udp
	DTSError			Write( const void * data, size_t size, int type );
	
	// return non-zero if connected
	bool				IsConnected() const;
	
	// return address of remote peer, if we know it
	DTSError			GetPeerAddress( DTSNetAddress& outAddr ) const;
	
	// close all endpoints
	void				Close();
};


/*
**	write type options
*/
enum
{
	kNetWriteReliable,
	kNetWriteUnreliable
};


/*
**	Errors
*/
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
** prototypes
*/
DTSError	DTSInitNetwork();				// initialize networking
void		DTSExitNetwork();				// close down networking
DTSError	DTSEthernetHardwareAddress( void * addr );	// get the 6 byte ethernet MAC address

#endif	// Network_dts_h

