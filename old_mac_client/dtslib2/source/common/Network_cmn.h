/*
**	Network_cmn.h		dtslib2
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

#ifndef NETWORK_CMN_H
#define NETWORK_CMN_H

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#if defined( DTS_Linux )
#  include "Network_linux.h"
#elif defined( DTS_Unix )
#  include "Network_unix.h"
#elif defined( DTS_XCODE ) || TARGET_API_MAC_OSX
#  include "Network_mach.h"
#else
#  error "Unknown platform."
#endif


/*
**	Platform implementation routines
*/
DTSError	DTSNetOpenTCP( DTSEndPoint * ep, ushort * port, bool bPassive );
DTSError	DTSNetOpenUDP( DTSEndPoint * ep, ushort * port, bool bPassive );
void		DTSNetClose( DTSEndPoint ep );
DTSError	DTSNetAlloc( DTSEndPoint ep, DTSNetCall ** call );
void		DTSNetFree( DTSNetCall * call );
DTSError	DTSNetListen( DTSEndPoint ep, DTSNetCall * call );
DTSError	DTSNetAccept( DTSEndPoint hostep, DTSEndPoint * chanep, DTSNetCall * call );
DTSError	DTSNetSend( DTSEndPoint ep, const void * data, size_t size );
DTSError	DTSNetReceive( DTSEndPoint ep, void * data, size_t * size );
DTSError	DTSNetUSend( DTSEndPoint ep, const DTSNetAddress& toaddr, const void * data, size_t datasize );
DTSError	DTSNetUReceive( DTSEndPoint ep, DTSNetAddress& fromaddr, void * data, size_t * datasize );
DTSError	DTSNetConnect( DTSEndPoint ep, const DTSNetAddress& hostaddr, DTSNetChannel * channel );
DTSError	DTSNetResolveName( DTSEndPoint ep, const char * str, DTSNetAddress& addr );

#if SEND_MULTI
// optional scatter-gather writes
DTSError	DTSNetSendMulti( DTSEndPoint ep,
							const void * d1, size_t s1,
							const void * d2, size_t s2 );
DTSError	DTSNetUSendMulti( DTSEndPoint ep, DTSNetAddress& toaddr,
							const void * d1, size_t s1,
							const void * d2, size_t s2 );
#endif	// SEND_MULTI

#if RCV_MULTI
// and corresponding reads (not yet implemented)
DTSError	DTSNetRcvMulti( DTSEndPoint ep, void * d1, size_t * s1, void * d2, size_t * s2 );
DTSError	DTSNetURcvMulti( DTSEndPoint ep, DTSNetAddress& fromaddr,
							void * d1, size_t * s1,
							void * d2, size_t * s2 );
#endif	// RCV_MULTI


/*
**	Variables
*/
extern int32_t gUniqueID;


#endif	// NETWORK_CMN_H
