/*
**	Network_mach.cp		CLServer Mac OS X
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

#include <sys/time.h>
#include <sys/types.h>
#include <sys/uio.h>

#include <netinet/tcp.h>

#include <netdb.h>
#include <unistd.h>

#include "Network_cmn.h"


/*
**	Definitions
*/

// set this to 1 for network event logging
// set it to 2 if you want partial packet-dumps as well
#ifndef DEBUG_VERSION_NETWORK
# define DEBUG_VERSION_NETWORK	0
#endif

// name of the file or device for network trace logs
// (/dev/console or /dev/stdout work, too)
#define NET_LOG_NAME		"net_debug.log"


// Map errno values into Mac OSStatus range
#define OSErrno()	(-3199 - errno)


// turn this on to enable functions needed by a client, but not by the server
#define CLIENT_SUPPORT		1

#if CLIENT_SUPPORT

// do we use IOKit for getting the machine's primary MAC address, or getifaddrs() et al.?
// If you choose to use IOKit, then apps that use this library -- and that
// call DTSEthernetHardwareAddress() -- must link against IOKit.framework.
# define GET_MAC_ADDR_FROM_IOKIT	0

# if GET_MAC_ADDR_FROM_IOKIT
#  include <IOKit/IOKitLib.h>
#  include <IOKit/network/IOEthernetInterface.h>
#  include <IOKit/network/IONetworkInterface.h>
#  include <IOKit/network/IOEthernetController.h>
# else
#  include <net/if.h>
#  include <net/if_dl.h>
#  include <net/ethernet.h>
#  include <netinet/in.h>
#  include <ifaddrs.h>
# endif  // GET_MAC_ADDR_FROM_IOKIT
#endif  // CLIENT_SUPPORT


/*
**	Internal Routines
*/
static DTSError		DTSNetSetOption( DTSEndPoint ep, int optLevel, int optName, int optValue );

#if DEBUG_VERSION_NETWORK
//static void		Lg( const char * fmt, ... ) PRINTF_LIKE(1, 2);
// if D_V_N > 1, then Dmp() dumps a block of memory; else it's a no-op
# if DEBUG_VERSION_NETWORK < 2
#  define Dmp( d, l )	((void) 0)
# endif
#endif	// DEBUG_VERSION_NETWORK


/*
**	Variables
*/

#if DEBUG_VERSION_NETWORK
static FILE *   dbgStream;
static bool     gNetDebugEnabled = false;
#endif	// DEBUG_VERSION_NETWORK


#if DEBUG_VERSION_NETWORK
/*
**	Lg()
**
**	low-level network debug progress reporting function
*/
static void
PRINTF_LIKE(1,2)
Lg( const char * fmt, ... )
{
	va_list va;
	va_start( va, fmt );
	if ( gNetDebugEnabled && dbgStream )
		{
		if ( (true) )	// show timestamp?
			{
			time_t now = time( nullptr );
			struct tm tm;
			localtime_r( &now,  &tm );
			fprintf( dbgStream, "%.4d/%.02d/%.02d %d:%.02d:%.02d ",
				tm.tm_year + 1900,
				tm.tm_mon + 1,
				tm.tm_mday,
				tm.tm_hour,
				tm.tm_min,
				tm.tm_sec );
			}
		vfprintf( dbgStream, fmt, va );
		}
	va_end( va );
//	fflush( dbgStream );
}


#if DEBUG_VERSION_NETWORK > 1
/*
**	Dmp()
**
**	prints a buffer as nicely formatted hex bytes and ascii
*/
static void
Dmp( const void * data, ssize_t len )
{
	const int BPL		= 16;		// bytes per line
	const int MaxToDump	= BPL * 4;	// limit to 4 lines
	
	// don't dump too much
	if ( len > MaxToDump )
		len = MaxToDump;
	
	const uchar * ptr = static_cast<const uchar *>( data );
	
	size_t offset = 0;
	ssize_t nleft = len;
	
	// do whole lines first
	while ( nleft > BPL )
		{
		// print the offset
		Lg( "%.4zX: ", offset );
		
		// print the hex
		for ( int i = 0; i < BPL; ++i )
			Lg( "%.2X ", ptr[ i ] );
		// and a gap
		Lg( "    " );
		// and the ascii characters
		for ( int i = 0; i < BPL; ++i )
			{
			char ch = (char) ptr[ i ];
			Lg( "%c", isprint(ch) ? ch : '.' );
			}
		
		// move to next line
		nleft  -= BPL;
		ptr    += BPL;
		offset += BPL;
		Lg( "\n" );
		}
	
	// one partial line of remainder, if any
	if ( nleft )
		{
		// print the offset
		Lg( "%.4zX: ", offset );
		// print the hex
		for ( int i = 0; i < nleft; ++i )
			Lg( "%.2X ", ptr[ i ] );
		// print the gap
		Lg( "%*s", (int)(4 + 3 * (BPL - nleft) ), " " );
		// print the characters
		for ( int i = 0; i < nleft; ++i )
			{
			char ch = (char) ptr[ i ];
			Lg( "%c", isprint(ch) ? ch : '.' );
			}
		Lg( "\n" );
		}
}
#endif	// DEBUG_VERSION_NETWORK > 1
#endif	// DEBUG_VERSION_NETWORK


/*
**	DTSInitNetwork()
**
**	initialize the network
*/
DTSError
DTSInitNetwork()
{
	// pick a random number for the unique identifier
	int id = int( GetRandom( 256 ) );
	id += id << 8;
	id += id << 16;
	// id *= 0x01010101;
	gUniqueID = id;
	
#if DEBUG_VERSION_NETWORK

        const char * env = getenv("CL_NET_DEBUG");
        if ( env && *env )
                gNetDebugEnabled = true;

        if ( gNetDebugEnabled )
                {
                // Finder launches us with current directory set to '/', which is generally not writable.
                // In that case, let's put the net-debug log file somewhere else -- e.g. in /tmp.
                char wd[ PATH_MAX + 1 ];
                getcwd( wd, sizeof wd );
//              fprintf( stderr, "CWD: %s\n", wd );

                const char * logfile = *env ? env : (wd[1] ? NET_LOG_NAME : "/tmp/" NET_LOG_NAME);
                dbgStream = fopen( logfile, "wa" );
                if ( not dbgStream )
                        {
                        id = errno;
                        fprintf( stderr, "Failed to open dbgStream: %s.\n", strerror( id ) );
                        return -id;
                        }

                if ( (true) )
                        setvbuf( dbgStream, nullptr, _IOLBF, 0 );

                Lg( "InitNetwork\n" );
                }
#endif  // DEBUG_VERSION_NETWORK
	
#if CLIENT_SUPPORT
	gNetworking = true;
#endif
	
	return noErr;
}


/*
**	DTSExitNetwork()
**
**	shutdown the network
*/
void
DTSExitNetwork()
{
#if DEBUG_VERSION_NETWORK
	Lg( "ExitNetwork\n" );
	if ( gNetDebugEnabled && dbgStream )
		{
		fclose( dbgStream );
		dbgStream = nullptr;
                gNetDebugEnabled = false;
		}
#endif	// DEBUG_VERSION_NETWORK
	
#if CLIENT_SUPPORT
	gNetworking = false;
#endif
}


/*
**	DTSNetOpen()
**
**	open and bind a TCP or UDP endpoint
**	return bound endpoint (socket) in *pep
**	if *pport != 0, bind to that port specifically.
**	return bound port in *pport.
**
**	for sockets, we do even more than the OT or XTI code:
**	- we'll do the actual listen() here as well, to convert
**		the socket into "listening mode".
**	- also, set the socket to non-blocking.
**
**	We assume that the port numbers are in native endian format.
*/
static DTSError
DTSNetOpen( DTSEndPoint * pep, ushort * pport, int type, bool bPassive )
{
	DTSError result = noErr;
	DTSEndPoint ep = kClosedEndPoint;
	in_port_t reqport = pport ? *pport : in_port_t(0);
	in_port_t retport = 0;
	
	// open an IP socket of the requested type and default protocol
	ep = socket( AF_INET, (IPPROTO_TCP == type) ? SOCK_STREAM : SOCK_DGRAM, IPPROTO_IP );
	if ( -1 == ep )
		result = OSErrno();
	
	// set it to non-blocking status
	if ( noErr == result )
		{
		int flags = fcntl( ep, F_GETFL, 0 );
		if ( -1 == flags )
			result = OSErrno();
		if ( noErr == result )
			{
			flags = fcntl( ep, F_SETFL, flags | O_NONBLOCK );
			if ( -1 == flags )
				result = OSErrno();
			}
		}
	
	// for the server's listening ports, turn on REUSEADDR option
	// to avoid EADDRINUSE errors or socket expiry delays upon restart
	if ( noErr == result
	&&   bPassive )
		{
		result = DTSNetSetOption( ep, SOL_SOCKET, SO_REUSEADDR, 1 );
		}
	
	// if a specific port was requested, bind socket to it (at the wildcard address)
	if ( noErr == result && reqport != 0 )
		{
		DTSNetAddress reqaddr;
		reqaddr.Set( AF_INET, INADDR_ANY, reqport );
		
		result = bind( ep, reqaddr.sa(), reqaddr.sa_len() );
		if ( 0 != result )
			result = kNetworkPortNotAvailable;
		}
	
	// if a specific port was requested,
	// determine what local port was actually assigned
	if ( noErr == result && reqport != 0 )
		{
		DTSNetAddress retaddr;
		
		// unlike XTI, bind() doesn't give it to us directly; have to ask for it
		socklen_t addrlen = retaddr.sa_len();
		result = getsockname( ep, retaddr.sa(), &addrlen );
		if ( noErr == result )
			{
			retport = retaddr.Port();
			
			// by here we would expect that retport == reqport || reqport == 0
			if ( 0       != reqport
			&&   retport != reqport )
				{
				result = kNetworkPortNotAvailable;
				}
			}
		
		// set the socket to listen for incoming connections
		// (only TCP, and only the accepting socket)
		if ( result >= 0 && IPPROTO_TCP == type && bPassive )
			{
			result = listen( ep, 8 );		// a reasonable backlog
			if ( noErr != result )
				result = OSErrno();
			}
		}
	
	// clean up on error
	if ( noErr != result )
		{
		DTSNetClose( ep );
		ep = kClosedEndPoint;
		retport = 0;
		}
	
	// inform caller
	if ( pep )
		*pep = ep;
	if ( pport )
		*pport = retport;
	
#if DEBUG_VERSION_NETWORK
	Lg( "NetOpen%s(%d) on [ANY]:%d; err %d\n",
		IPPROTO_TCP == type ? "TCP" : "UDP", ep,
		ntohs(retport), int(result) );
#endif
	
	return result;
}


/*
**	DTSNetOpenTCP()
**
**	open and bind a TCP endpoint
*/
DTSError
DTSNetOpenTCP( DTSEndPoint * pep, ushort * pport, bool bPassive )
{
	assert( pep );
	DTSError result = DTSNetOpen( pep, pport, IPPROTO_TCP, bPassive );
	
	// turn on keepalive; reduce keepalive tickle time to 1 hr (default is 2hrs)
	if ( noErr == result && not bPassive )
		{
		result = DTSNetSetOption( *pep, SOL_SOCKET, SO_KEEPALIVE, 1 );
		if ( noErr == result )
			result = DTSNetSetOption( *pep, IPPROTO_TCP, TCP_KEEPALIVE, 60 * 60 );
		}
	
	// never interested in reading anything less than 2 bytes from TCP
	if ( noErr == result )
		result = DTSNetSetOption( *pep, SOL_SOCKET, SO_RCVLOWAT, 2 );
	
	return result;
}


/*
**	DTSNetOpenUDP()
**
**	open and bind a UDP endpoint
*/
DTSError
DTSNetOpenUDP( DTSEndPoint * pep, ushort * pport, bool bPassive )
{
	DTSError result = DTSNetOpen( pep, pport, IPPROTO_UDP, bPassive );
	return result;
}


/*
**	DTSNetClose()
**
**	close an endpoint
*/
void
DTSNetClose( DTSEndPoint ep )
{
	if ( not IsEndPointOpen( ep ) )
		return;
	
	int result = close( ep );
	if ( result < 0 )
		result = OSErrno();
	
#if DEBUG_VERSION_NETWORK
	Lg( "NetClose(%d) err %d %s\n", ep,
		 result, result < 0 ? strerror( errno ) : "OK" );
#else
	(void) result;
#endif  // DEBUG_VERSION_NETWORK
}


/*
**	DTSNetAlloc()
**
**	allocate a listen call structure
*/
DTSError
DTSNetAlloc( DTSEndPoint /*ep*/, DTSNetCall ** /*pcall*/ )
{
	// since BSD sockets doesn't use the concept of net calls,
	// we could cheat here, and use them as a handy packet of
	// purely local state. That might allow Listen(), Connect(),
	// Accept(), and crew to manage the sockets more effectively.
	// ... just an idea.
	
	return noErr;
}


/*
**	DTSNetFree()
**
**	free a listen call structure
*/
void
DTSNetFree( DTSNetCall * /*call*/ )
{
	// a no-op for sockets API
}


/*
**	DTSNetListen()
**
**	listen for new TCP connections
*/
DTSError
DTSNetListen( DTSEndPoint ep, DTSNetCall * /*call*/ )
{
	// for sockets, we already listen()'ed when opening the socket
	// so here, we'll just test whether a new connection is available
	DTSError result = noErr;
	
	// set up the read descriptor map
	fd_set fds;
	FD_ZERO( &fds );
	FD_SET( ep, &fds );
	
	// return immediately if socket not readable
	timeval tv;
	timerclear( &tv );
	
	// test readability of the socket
	int nReadable = select( ep + 1, &fds, nullptr, nullptr, &tv );
	
	// handle errors
	if ( nReadable < 0 )
		{
		if ( EAGAIN == errno || EINTR == errno )
			result = 1;	 // pretend there's no data
		else
			result = OSErrno();
		}
	else
	if ( not FD_ISSET( ep, &fds ) )
		result = 1;						// meaning "nothing to read"
	
#if DEBUG_VERSION_NETWORK
	// avoid spam
	if ( result < 1 )
		Lg( "Lstn(%d) err %d\n", ep, int(result) );
#endif
	
	return result;
}


/*
**	DTSNetAccept()
**
**	accept new TCP connections
*/
DTSError
DTSNetAccept( DTSEndPoint hostep, DTSEndPoint * chanep, DTSNetCall * /*call*/ )
{
	// paranoia
	if ( not chanep )
		return -1;
	
	DTSError result = noErr;
	
	// prepare the result to hold the connectee's address
	DTSNetAddress addr;
	socklen_t addrlen = addr.sa_len();
	
	// accept a connection (and receive a new descriptor from kernel)
	int newfd = accept( hostep, addr.sa(), &addrlen );
	if ( newfd < 0 )
		{
		if ( EAGAIN == errno )
			result = kNetworkNoData;
		else
			result = OSErrno();
		}
	
	// hang onto the new descriptor
	*chanep = newfd;
	
	// set the non-blocking flag
	int flags = 0;
	if ( noErr == result )
		{
		flags = fcntl( *chanep, F_GETFL, 0 );
		if ( flags < 0 )
			result = OSErrno();
		}
	if ( noErr == result && 0 == (flags & O_NONBLOCK) )
		{
		flags = fcntl( *chanep, F_SETFL, flags | O_NONBLOCK );
		if ( flags < 0 )
			result = OSErrno();
		}
	
# if DEBUG_VERSION_NETWORK
	if ( kNetworkNoData != result )
		{
		char buf[ INET6_ADDRSTRLEN ];
		addr.ToString( buf, sizeof buf );
		Lg( "Acc(%d) from %s:%d -> socket %d; err %d\n", hostep,
			  buf, addr.Port(),
			  *chanep, int(result) );
		}
# endif	 // DEBUG_VERSION_NETWORK
	
	return result;
}


/*
**	DTSNetSend()
**
**	send data over TCP
*/
DTSError
DTSNetSend( DTSEndPoint ep, const void * data, size_t sz )
{
	DTSError result = noErr;
	size_t nleft = sz;
	const char * ptr = static_cast<const char *>( data );
	
	while ( nleft > 0  && noErr == result )
		{
		// send what we can. We may not be able to send the
		// entire buffer all at once; if so, just keep on
		// sending whatever increments it'll take.
		ssize_t nwritten = write( ep, ptr, nleft );
		
		// but if there was an error...
		if ( nwritten < 0 )
			{
			nwritten = 0;
			
			// got a signal?
			if ( EINTR == errno )
				;	// just try again
			else
			if ( EAGAIN == errno )
				result = kNetworkPartialSend;
			else
				{
				// something nasty happened
#if DEBUG_VERSION_NETWORK
				Lg( "TCPSend(%d) failed (%d): %s\n", ep, errno, strerror( errno ) );
#endif
				result = kNetworkDisconnect;
				}
			}
		
		// remember the amount written so far
		nleft -= static_cast<size_t>( nwritten );
		ptr += nwritten;
		}
	
	// by here, either the whole packet was sent, or there was an error.
#if DEBUG_VERSION_NETWORK
	Lg( "TCPSend(%d): sent %zub of %zu, err %d\n", ep, sz - nleft, sz, int(result) );
	Dmp( data, sz - nleft );
#endif
	
	return result;
}


#if SEND_MULTI
/*
**	DTSNetSendMulti()
**
**	send data from two separate buffers over TCP
**	nearly all client-server packets start with a 2-byte length field,
**	followed by that many bytes of 'payload' (frame data). To send such
**	packets using plain DTSNetSend(), an intermediate buffer is needed,
**	so that the length and data can be contiguous. That involves at least
**	one memory allocation, one memcpy(), and one deallocation per packet.
**	But there's a better way! If we can use scatter-gather reads and writes,
**	we can avoid the memory churn and the memcpy() altogether.
**	It turns out that both OpenTransport and the sockets() API can do it,
**	although the Sun's version of XTI/Streams is too old.
**
**	Anyway, this routine is just like DTSNetSend() except that it combines
**	's1' bytes at 'd1', and 's2' bytes at 'd2', into one packet.
*/
DTSError
DTSNetSendMulti( DTSEndPoint ep, const void * d1, size_t s1, const void * d2, size_t s2 )
{
	DTSError result = noErr;
	
	iovec v[ 2 ];
	v[0].iov_base = const_cast<void *>( d1 );
	v[0].iov_len  = s1;
	v[1].iov_base = const_cast<void *>( d2 );
	v[1].iov_len  = s2;
	
	ssize_t nwritten = writev( ep, v, 2 );
	
	// but if there was an error...
	if ( nwritten < 0 )
		{
		// got a signal?
		if ( EINTR  == errno
		||   EAGAIN == errno )
			{
			result = kNetworkPartialSend;
			}
		else
			{
			// something nasty happened
# if DEBUG_VERSION_NETWORK
			Lg( "TCPSendMulti(%d) failed (%d): %s\n", ep, errno, strerror( errno ) );
# endif
			result = kNetworkDisconnect;
			}
		}
	
	// by here, either the whole packet was sent, or there was an error.
# if DEBUG_VERSION_NETWORK
	Lg( "TCPSendMulti(%d): sent %zdb of %zu, err %d\n", ep,
		nwritten, s1 + s2, int(result) );
	Dmp( d1, s1 - nwritten );
	Dmp( d2, s2 - nwritten );
# endif
	
	return result;
}
#endif	// SEND_MULTI


/*
**	DTSNetReceive()
**
**	receive data from TCP
*/
DTSError
DTSNetReceive( DTSEndPoint ep, void * data, size_t * sz )
{
	DTSError result = noErr;
	ssize_t numread = read( ep, data, *sz );

	// handle "eof" -- remote end closed the stream
	if ( 0 == numread )
		{
		result = close( ep );
#if DEBUG_VERSION_NETWORK
		if ( -1 == result )
			result = errno;
		Lg( "TCPRcv(%d): got 0b. Closing socket, err %d %s\n",
				ep, int(result),
				result == 0 ? "" : strerror(result) );
#else
		(void) result;
#endif	// DEBUG_VERSION_NETWORK
		
		return kNetworkDisconnect;
		}
	
	// handle read errors
	if ( numread < 0 )
		{
		numread = 0;
		// call was interrupted by a signal
		if ( EINTR == errno )
			result = kNetworkNoData;		// we presume the upper layer
											// will eventually get around to
											// calling read() again to pick up
											// where we left off.
		else
		// no data available
		if ( EAGAIN == errno )			// this constant is also called EWOULDBLOCK
			result = kNetworkNoData;
		else
			{
			// any other error is bad news. Assume that the connection is now dead
#if DEBUG_VERSION_NETWORK
			Lg( "DTSNetReceive failed (%d): %s\n", errno, strerror( errno ) );
#endif
			result =  kNetworkDisconnect;
			}
		}
	
#if DEBUG_VERSION_NETWORK
	// skip the common case, log everything else
	if ( numread != 0
	||	 *sz != 2
	||	 result != kNetworkNoData )
		{
		Lg( "TCPRcv(%d): got %zdb of %zu, err %d\n", ep, numread, *sz, int(result) );
		}
	Dmp( data, numread );
#endif	// DEBUG_VERSION_NETWORK
	
	// return amount read and result
	*sz = static_cast<size_t>( numread );
	return result;
}


#if RCV_MULTI
/*
**	DTSNetRcvMulti()
**
**	scatter-gather TCP read
*/
DTSError
DTSNetRcvMulti( DTSEndPoint ep, void * d1, size_t * s1, void * d2, size_t * s2 )
{
	DTSError result = noErr;
	iovec v[ 2 ];
	
	v[ 0 ].iov_base = d1;
	v[ 0 ].iov_len  = *s1;
	v[ 1 ].iov_base = d2;
	v[ 1 ].iov_len  = *s2;
	
	// read into both buffers
	// (unless we're asking for so little data that it'd all fit into d1)
	ssize_t numread = readv( ep, v, ( d2 && *s2 > 0 ) ? 2 : 1 );
	
	// handle "eof" -- remote end closed the stream
	if ( 0 == numread )
		{
		result = close( ep );
#if DEBUG_VERSION_NETWORK
		if ( -1 == result )
			result = errno;
		Lg( "TCPRcvMulti(%d): got 0b (EOF). Closing socket, err %d %s\n",
				ep, int(result),
				result == 0 ? "" : strerror(result) );
#else
		(void) result;
#endif	// DEBUG_VERSION_NETWORK
		
		return kNetworkDisconnect;
		}
	
	// handle read errors
	if ( numread < 0 )
		{
		numread = 0;
		// call was interrupted by a signal
		if ( EINTR == errno )
			result = kNetworkNoData;		// we presume the upper layer
											// will eventually get around to
											// calling read() again to pick up
											// where we left off.
		else
		// no data available
		if ( EAGAIN == errno )			// this constant is also called EWOULDBLOCK
			result = kNetworkNoData;
		else
			{
			// any other error is bad news. Assume that the connection is now dead
#if DEBUG_VERSION_NETWORK
			Lg( "TCPRcvMulti failed (%d): %s\n", errno, strerror( errno ) );
#endif
			result =  kNetworkDisconnect;
			}
		}
	
#if DEBUG_VERSION_NETWORK
	// skip the common case, log everything else
	if ( numread != 0
//	||	 *sz != 2
	||	 result != kNetworkNoData )
		{
		Lg( "TCPRcvMulti(%d): got %zdb of %zu, err %d\n", ep,
			numread, *s1 + *s2, int(result) );
		}
	
# if DEBUG_VERSION_NETWORK > 1
	size_t toDump = *s1;
	if ( ssize_t(toDump) > numread )
		toDump = numread;
	Dmp( d1, toDump );
	toDump = numread - toDump;
	Dmp( d2, toDump );
# endif  // DEBUG_VERSION_NETWORK > 1
#endif	// DEBUG_VERSION_NETWORK
	
	// return amounts read and result.
	// [by here, numread is always >= 0]
	if ( size_t(numread) < *s1 )
		{
		*s1 = static_cast<size_t>( numread );
		*s2 = 0;
		}
	else
		{
//		*s1 = *s1;
		*s2 = static_cast<size_t>( numread ) - *s1;
		}
	return result;
}
#endif  // RCV_MULTI


/*
**	DTSNetUSend()
**
**	send UDP data
*/
DTSError
DTSNetUSend( DTSEndPoint ep, const DTSNetAddress& toaddr, const void * data, size_t sz )
{
	DTSError result = noErr;
	
	// the kernel might not accept the whole packet in a single call; so
	// we have to be prepared to dole the data out in smaller dribbles.
	size_t nleft = sz;
	const char * writepos = static_cast<const char *>( data );
	
	// send it
	while ( nleft > 0 && noErr == result )
		{
		// attempt to transmit the entire remaining contents of the buffer
		ssize_t nsent = sendto( ep, writepos, nleft, 0, toaddr.sa(), toaddr.sa_len() );
		
		// handle errors
		if ( nsent < 0 )
			{
			nsent = 0;		// if error, assume that no data was actually sent
			
			// some errors are OK, some are not.
			if ( EAGAIN == errno )			// aka EWOULDBLOCK
				{
				// try again later?
#if DEBUG_VERSION_NETWORK
				Lg( "DTSNetUSend(%d) got EAGAIN -- why?!?\n", ep );
#endif
				result = kNetworkNoData;	// huh?
				}
			else
			if ( EINTR == errno )
				// signalus interruptus: try again right now.
				continue;
			else
				{
				// something worse happened
#if DEBUG_VERSION_NETWORK
				result = errno;
				Lg( "DTSNetUSend(%d) failed (%d): %s\n", ep, int(result), strerror(result) );
#endif
				result = kNetworkUnitData;
				}
			}
		
		// update amount left to send
		nleft -= static_cast<size_t>( nsent );
		writepos += nsent;
		}
	
#if DEBUG_VERSION_NETWORK
	{
	char add[ INET6_ADDRSTRLEN ];
	toaddr.ToString( add, sizeof add );
	
	Lg( "UDPSend(%d): sent %zd of %zub to %s:%d err %d\n",
			ep, sz - nleft, sz,
			add, toaddr.Port(), int(result) );
	Dmp( data, sz );
	}
#endif	// DEBUG_VERSION_NETWORK
	
	return result;
}


#if SEND_MULTI
/*
**	DTSNetUSendMulti()
**
**	send UDP data from 2 buffers
**	See DTSNetSendMulti() above for rationale.
*/
DTSError
DTSNetUSendMulti( DTSEndPoint ep, DTSNetAddress& toaddr,
	const void * d1, size_t s1, const void * d2, size_t s2 )
{
	DTSError result = noErr;
	
	iovec v[ 2 ];
	v[0].iov_base = const_cast<void *>( d1 );
	v[0].iov_len  = s1;
	v[1].iov_base = const_cast<void *>( d2 );
	v[1].iov_len  = s2;
	
	msghdr msg;
	memset( &msg, 0, sizeof msg );
	msg.msg_name    = toaddr.sa();
	msg.msg_namelen = toaddr.sa_len();
	msg.msg_iov     = v;
	msg.msg_iovlen  = 2;
	
	// send it
	ssize_t nsent = sendmsg( ep, &msg, 0 );
	
	// handle errors
	if ( nsent < 0 )
		{
		if ( EAGAIN == errno			// aka EWOULDBLOCK
		||   EINTR  == errno )
			{
			result = kNetworkPartialSend;
			}
		else
			{
			// something worse happened
# if DEBUG_VERSION_NETWORK
			result = errno;
			Lg( "DTSNetUSendMulti(%d) failed (%d): %s\n", ep, int(result), strerror(result) );
# endif
			result = kNetworkUnitData;
			}
		}
	
# if 0 && DEBUG_VERSION_NETWORK
	{
	char adr[ INET6_ADDRSTRLEN ];
	toaddr.ToString( toaddr, adr, sizeof adr );
	Lg( "UDPSendMulti(%d): sent %zd of %zub to %s:%d err %d\n",
			ep, nsent, s1 + s2,
			adr, toaddr.Port(), int(result) );
	Dmp( d1, s1 );
	Dmp( d2, s2 );
	}
# endif  // DEBUG_VERSION_NETWORK
	
	return result;
}
#endif  // SEND_MULTI


/*
**	DTSNetUReceive()
**
**	receive UDP data
*/
DTSError
DTSNetUReceive( DTSEndPoint ep, DTSNetAddress& wantaddr, void * data, size_t * sz )
{
	// this is the amount of data we'll report back to our caller.
	// might be different from the true amount, if the remote end
	// isn't who we're expecting it to be.
	size_t numread = 0;
	
	DTSError result = noErr;
	
	// get the address of the remote end
	// (recvfrom() only writes into this; it doesn't examine it before the
	// read, nor use the address as any kind of filter on which datagrams it'll accept.)
	DTSNetAddress sa;
	
	// try to read some data
recv_again:
	enum { kNilFlags = 0 };
	socklen_t len = sa.sa_len();
	ssize_t nread = recvfrom( ep, data, *sz, kNilFlags, sa.sa(), &len );
	
	// handle read errors
	if ( nread < 0 )
		{
		nread = 0;
		// some errors are benign
		if ( EINTR == errno	)
			// interrupted by a signal -- just retry the read
			goto recv_again;
		else
		if ( EAGAIN == errno )		// no data available
			result = kNetworkNoData;
		else
			{
			// any other error is bad. log it for now.
#if DEBUG_VERSION_NETWORK
			Lg( "DTSNetUReceive(%d) failed (%d): %s\n", ep, errno, strerror(errno) );
#endif
			result = kNetworkDisconnect;
			}
		}
	
	if ( noErr == result )
		{
#if 0 && DEBUG_VERSION_NETWORK
		char sourceIP[ INET6_ADDRSTRLEN ], wantIP[ INET6_ADDRSTRLEN ];
		sa.ToString( sourceIP, sizeof sourceIP );
		wantaddr.ToString( wantIP, sizeof wantIP );
		Lg( "UDPRcv(%d): got %zdb from [%d]%s:%d;"
			   " wanted %zu from [%d]%s:%d.\n", ep,
			   nread, sa.Family(), sourceIP, sa.Port(),
			   *sz, wantaddr.Family(), wantIP, wantaddr.Port() );
		Dmp( data, nread );
#endif  // DEBUG_VERSION_NETWORK
		
		// make sure we got the data from where we wanted to
		// (which includes the case where we'll accept from anybody)
		if ( wantaddr.Family() != AF_INET
		||   sa == wantaddr )
			{
			numread = static_cast<size_t>( nread );
			
			// save the address if we need to
			if ( AF_UNSPEC == wantaddr.Family() )	// i.e., not explicitly set
				wantaddr = sa;
			}
		}
	
	// return the number read and the result
	*sz = numread;
	return result;
}


#if RCV_MULTI
/*
**	DTSNetURcvMulti()
**
**	scatter-gather UDP read
*/
DTSError
DTSNetURcvMulti( DTSEndPoint ep, DTSNetAddress& fromaddr,
				 void * d1, size_t * s1, void * d2, size_t * s2 )
{
	DTSError result = noErr;
	
	iovec v[ 2 ];
	v[ 0 ].iov_base = d1;
	v[ 0 ].iov_len  = *s1;
	v[ 1 ].iov_base = d2;
	v[ 1 ].iov_len  = *s2;
	
	DTSNetAddress sa;
	
	msghdr mh;
	memset( &mh, 0, sizeof mh );
	mh.msg_name = sa.sa();
	mh.msg_namelen = sa.sa_len();
	mh.msg_iov = v;
	mh.msg_iovlen = (*s2 > 0) ? 2 : 1;
	
	ssize_t numread = recvmsg( ep, &mh, 0 );
	if ( -1 == numread )
		{
		int err = errno;
		if ( EINTR == err
		||   EAGAIN == err )
			{
			return kNetworkNoData;
			}
		
		// any other error is bad. log it for now.
#if DEBUG_VERSION_NETWORK
		Lg( "DTSNetURcvMulti(%d) failed (%d): %s\n", ep, err, strerror(err) );
#endif
		result = kNetworkDisconnect;
		
		numread = 0;
		}
	
	if ( noErr == result )
		{
		// make sure we got the data from where we wanted to
		// (which includes the case where we'll accept from anybody)
		
		if ( fromaddr.Family() != AF_INET
		||	 fromaddr == sa )
			{
			// save the address if we need to
			if ( AF_UNSPEC == fromaddr.Family() )
				fromaddr = sa;
			}
		}
	
	// return the number read and the result
	if ( numread > static_cast<ssize_t>( *s1 ) )
		{
		*s2 = static_cast<size_t>( numread ) - *s1;
		}
	else
		{
		*s1 = static_cast<size_t>( numread );
		*s2 = 0;
		}
	
	return result;
}
#endif  // RCV_MULTI


/*
**	DTSNetSetOption()
**
**	twiddle a socket option (max option value size = 32 bits) 
*/
DTSError
DTSNetSetOption( DTSEndPoint ep, int optLevel, int optName, int optValue )
{
	int result = setsockopt( ep, optLevel, optName, &optValue, sizeof optValue );
	
	if ( noErr != result )
		result = OSErrno();
	
	return result;
}


/*
**	DTSNetConnect()
**
**	connect to a host
**	not needed by server
*/
DTSError
DTSNetConnect( DTSEndPoint ep, const DTSNetAddress& hostaddr, DTSNetChannel * channel )
{
#if not CLIENT_SUPPORT
# pragma unused( ep, hostaddr, channel )
	return -1;
#else
	// NB: this is a really minimalist version.
	// Better code would put the networking into its own thread, or, at the very least,
	// schedule the connect() on a runloop so that the UI stays responsive.
	
	// the socket is in non-blocking mode, so the connect() will very likely
	// return before the connection itself is fully established.
	DTSError result = connect( ep, hostaddr.sa(), hostaddr.sa_len() );
	if ( -1 == result )
		result = errno; // OSErrno();
	
	// allow a maximum of 10 seconds for the connection to be made.
	// while we're waiting, tickle the callback once per second.
	uint nSeconds = 0;
	while ( EINPROGRESS == result )
		{
		if ( ++nSeconds > 10 )
			return kNetworkHandshakeTimeout;
		
		// inform user; allow him to cancel.
		result = channel->ConnectCallBack( "Waiting for host." );
		if ( noErr != result )
			return result;
		
		// the socket will become writable as soon as we're connected
		fd_set fds;
		FD_ZERO( &fds );
		FD_SET( ep, &fds );
		timeval tv;
		tv.tv_sec  = 1;
		tv.tv_usec = 0;
		int nfds = select( ep + 1, nullptr, &fds, nullptr, &tv );
		if ( -1 == nfds )
			{
			result = errno; // OSErrno();
			if ( EAGAIN == result )
				{
				result = EINPROGRESS;	// stay in the while loop
				continue;
				}
			// anything else is presumed fatal
			}
		else
		if ( 1 == nfds && FD_ISSET( ep, &fds ) )
			return noErr;		// got it!
		}
	
	// convert all errors into a catch-all
	if ( noErr != result )
		result = kNetworkDisconnect;
	
	return result;
	
#endif  // CLIENT_SUPPORT
}


/*
**	DTSNetResolveName()
**
**	resolve the domain name to an IPv4 address
**	[client only]
*/
DTSError
DTSNetResolveName( DTSEndPoint /*ep*/, const char * str, DTSNetAddress& oAddr )
{
#if not CLIENT_SUPPORT
# pragma unused( str, oAddr )
	return -1;
#else
	// presume failure
	DTSError result = kNetworkBadTCPAddress;
	
	// prophylaxis
	if ( not str )
		return result;
	
	// start with clean slate
	oAddr.Reset();
	
	// give a hint as to what sort of addresses we want
	addrinfo hint;
	memset( &hint, 0, sizeof hint );
	hint.ai_family   = AF_INET;
	hint.ai_socktype = SOCK_STREAM;
	hint.ai_protocol = IPPROTO_TCP;
	
	// look it up
	addrinfo * addressList = nullptr;
	int res = getaddrinfo( str, nullptr, &hint, &addressList );
	if ( res != 0 )
		{
# if DEBUG_VERSION_NETWORK
		Lg( "ResolveName: '%s' failed (%d) - %s\n", str, res, gai_strerror( res ) );
# endif
		return result;
		}
	
	// grab the first valid result
	for ( const addrinfo * test = addressList; test; test = test->ai_next )
		{
		// [redundant] test for correct family
		if ( AF_INET == test->ai_family )
			{
			oAddr.Set( test->ai_addr );
			result = noErr;
			break;
			}
		}
	if ( addressList )
		freeaddrinfo( addressList );
	
# if DEBUG_VERSION_NETWORK
	{
	char buf[ INET6_ADDRSTRLEN ];
	oAddr.ToString( buf, sizeof buf );
	Lg( "ResolveName: %d - '%s' is [%d]%s.\n", res,
		str,
		oAddr.Family(),
	    buf );
	}
# endif	// DEBUG_VERSION_NETWORK
	
	return result;
#endif	// CLIENT_SUPPORT
}


#if CLIENT_SUPPORT

# if GET_MAC_ADDR_FROM_IOKIT

// largely stolen from <http://developer.apple.com/samplecode/GetPrimaryMACAddress/index.html>


// Returns an iterator containing the primary (built-in) Ethernet interface.
// The caller is responsible for releasing the iterator after the caller is done with it.
static kern_return_t
FindEthernetInterfaces( io_iterator_t * matchingServices )
{
	CFMutableDictionaryRef matchDict = IOServiceMatching( kIOEthernetInterfaceClass );
	if ( not matchDict )
		return KERN_FAILURE;
	
	CFMutableDictionaryRef propDict = CFDictionaryCreateMutable(
		kCFAllocatorDefault, 0,
		&kCFTypeDictionaryKeyCallBacks,
		&kCFTypeDictionaryValueCallBacks );
	if ( not propDict )
		{
		CFRelease( matchDict );
		return KERN_FAILURE;
		}
	
	CFDictionarySetValue( propDict, CFSTR(kIOPrimaryInterface), kCFBooleanTrue );
	
	CFDictionarySetValue( matchDict, CFSTR(kIOPropertyMatchKey), propDict );
	CFRelease( propDict );
	
	kern_return_t result = IOServiceGetMatchingServices( kIOMasterPortDefault,
			matchDict, matchingServices );
	return result;
}


// Given an iterator across a set of Ethernet interfaces, return the MAC address of the last one.
// If no interfaces are found the MAC address is set to an empty string.
// In this sample the iterator should contain just the primary interface.
static kern_return_t
GetMACAddress( io_iterator_t iter, UInt8 addr[ kIOEthernetAddressSize ] )
{
	kern_return_t result = KERN_FAILURE;
	
	while ( io_object_t intfService = IOIteratorNext( iter ) )
		{
		io_object_t controllerService;
		result = IORegistryEntryGetParentEntry( intfService,
						   kIOServicePlane,
						   &controllerService );
		
		if ( KERN_SUCCESS == result )
			{
			// Retrieve the MAC address property from the I/O Registry in the form of a CFData
			CFTypeRef d = IORegistryEntryCreateCFProperty( controllerService,
								 CFSTR(kIOMACAddress),
								 kCFAllocatorDefault,
								 kNilOptions );
			if ( d /* && CFDataGetTypeID() == CFGetTypeID( d ) */ )
				{
				CFDataGetBytes( static_cast<CFDataRef>( d ),
					CFRangeMake( 0, kIOEthernetAddressSize ), addr );
				CFRelease( d );
				}
			
			// Done with the parent Ethernet controller object so we release it.
			(void) IOObjectRelease( controllerService );
			}
		
		// Done with the Ethernet interface object so we release it.
		(void) IOObjectRelease( intfService );
		}
	
	return result;
}


# elif 0
/*
**	Another method to get the same thing (I think)
**	Taken from <http://developer.apple.com/devcenter/mac/documents/validating.html>
**	although there they sometimes glorify it as the machine's GUID, rather than
**	a mere MAC address...
*/
static CFDataRef NOT_USED
CopyMACAddress()
{
	CFMutableDictionaryRef matcher = IOBSDNameMatching( kIOMasterPortDefault, 0, "en0" );
	if ( not matcher )
		return nullptr;
	
	io_iterator_t iter;
	kern_return_t result = IOServiceGetMatchingServices( kIOMasterPortDefault, matcher, &iter );
	if ( KERN_SUCCESS != result )
		{
		CFRelease( matcher );
		return nullptr;
		}
	
	io_object_t service;
	CFDataRef macAddress = nullptr;
	while ( (service = IOIteratorNext( iter )) != 0 )
		{
		io_object_t parent;
		result = IORegistryEntryGetParentEntry( service, kIOServicePlane, &parent );
		if ( KERN_SUCCESS == result )
			{
			if ( macAddress )
				CFRelease( macAddress );
			
			macAddress = static_cast<CFDataRef>( IORegistryEntryCreateCFProperty( parent,
				CFSTR(kIOMACAddress), kCFAllocatorDefault, kNilOptions ) );
			IOObjectRelease( parent );
			}
		
		IOObjectRelease( iter );
		IOObjectRelease( service );
		}
	
	return macAddress;
}


#else  // ! GET_MAC_ADDR_FROM_IOKIT

/*
**	yet another way; a somewhat homegrown one that doesn't need IOKit
**	(but might not give exactly the same answer as the other methods, at least
**	on systems with more complicated network configurations)
*/
static OSStatus
MyEthernetHardwareAddress( void * addr, uint addrLen )
{
	OSStatus result = paramErr;		// pessimism
	
	// paranoia
	if ( not addr || addrLen < ETHER_ADDR_LEN )
		return result;
	memset( addr, 0, addrLen );			// more pessimism
	
	// get interface list
	ifaddrs * addrs = nullptr;	// linked list of all network interfaces
	int rc = getifaddrs( &addrs );
	if ( 0 != rc )
		return OSErrno();
	
	// now iterate the list, weeding out unsuitable candidates
	for ( const ifaddrs * ip = addrs; ip; ip = ip->ifa_next )
		{
		// skip those that have no address
		if ( not ip->ifa_addr )
			continue;
		
		// skip the loopback interface
		if ( ip->ifa_flags & IFF_LOOPBACK )
			continue;
		
		// skip interfaces that are down
		if ( not (ip->ifa_flags & IFF_UP) )
			continue;
		
		// skip those that aren't in the right family, namely IF_LINK
		if ( ip->ifa_addr->sa_family != AF_LINK )
			continue;
		
		// this could be the one!
		// at any rate, it should be good enough for guv'mint work
		const sockaddr_dl * dladdr = reinterpret_cast<sockaddr_dl *>( ip->ifa_addr );
		
		// ... except if there's no hardware address at all
		if ( 0 == dladdr->sdl_alen )
			continue;
		
		// grab the address (but not more of it than we have room for)
		if ( addrLen > dladdr->sdl_alen )
			addrLen = dladdr->sdl_alen;
		memcpy( addr, LLADDR( dladdr ), addrLen );
		
		// and we are done!
		result = noErr;
		break;
		}
	
	// release the interface list
	if ( addrs )
		freeifaddrs( addrs );
	
	return result;
}
# endif  // GET_MAC_ADDR_FROM_IOKIT
#endif  // CLIENT_SUPPORT


/*
**	DTSEthernetHardwareAddress()
**
**	get primary enet MAC address
**	not needed by server
**	Doesn't really belong here anyway
*/
DTSError
DTSEthernetHardwareAddress( void * oAddr )
{
#if not CLIENT_SUPPORT
# pragma unused( oAddr )
	return -1;
#else
# if GET_MAC_ADDR_FROM_IOKIT
	uint8_t addr[ kIOEthernetAddressSize ] = {};
	
	io_iterator_t iter;
	kern_return_t result = FindEthernetInterfaces( &iter );
	if ( KERN_SUCCESS == result )
		result = GetMACAddress( iter, addr );
	(void) IOObjectRelease( iter );  // Release the iterator.
	
	memcpy( oAddr, addr, sizeof addr );
	
    return result;
# else
	return MyEthernetHardwareAddress( oAddr, ETHER_ADDR_LEN );
# endif  // GET_MAC_ADDR_FROM_IOKIT
#endif  // CLIENT_SUPPORT
}

