/*
**	Network_mach.h		CLServer Mac OS X
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

#ifndef NETWORK_MACH_H
#define NETWORK_MACH_H

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include <sys/types.h>
#include <sys/socket.h>
#include <sys/select.h>
#include <netinet/in.h>
#include <arpa/inet.h>


/*
**	Definitions
*/

// network endpoint
typedef	int		DTSEndPoint;		// a socket file-descriptor

// Auxiliary XTI data structure, not used by sockets API, but we still need a typedef for it
typedef void	DTSNetCall;

// sentinel value for an endpoint that has never been opened, or has been closed
const DTSEndPoint kClosedEndPoint = -1;


/*
**	IsEndPointOpen()
*/
inline bool
IsEndPointOpen( DTSEndPoint ep )
{
	return ep > 0;		// (this also includes STDERR_FILENO et al, so may be too broad)
}

// Darwin/OSX does support scatter-gather writes and reads. Woot!
#define	SEND_MULTI			1
#define	RCV_MULTI			1

#endif	// NETWORK_MACH_H

