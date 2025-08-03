/*
**	Network_linux.cp		dtslib2
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
#include "Prefix_dts.h"
#endif

#include "Network_cmn.h"



/*
**	Definitions
*/


/*
**	Internal Routines
*/


/*
**	Variables
*/


/*
**	DTSInitNetwork()		+++	Unix Sockets +++
**
**	initialize the network
*/
DTSError
DTSInitNetwork()
{
	gNetworking = true;
	
	return result;
}


/*
**	DTSExitNetwork()		+++	Unix Sockets +++
**
**	shutdown the network
*/
void
DTSExitNetwork()
{
	gNetworking = false;
}


/*
**	DTSNetOpenTCP()		+++	Unix Sockets +++
**
**	open and bind a tcp end point
*/
DTSError
DTSNetOpenTCP( DTSEndPoint * pep, long * pport )
{
	DTSError result = DTSNetOpen( pep, pport, kTCPName );
	return result;
}


/*
**	DTSNetOpenUDP()		+++	Unix Sockets +++
**
**	open and bind a udp end point
*/
DTSError
DTSNetOpenUDP( DTSEndPoint * pep, long * pport )
{
	DTSError result = DTSNetOpen( pep, pport, kUDPName );
	return result;
}


/*
**	DTSNetOpen()		+++	Unix Sockets +++
**
**	open and bind a tcp or udp end point
*/
static DTSError
DTSNetOpen( DTSEndPoint * pep, long * pport, char * path )
{
	mSocket		= socket(mDomain, mType, mProtocol);
	
	InetAddress reqaddr;
	TBind reqbind;
	InetAddress retaddr;
	TBind retbind;
	
	DTSEndPoint ep = kClosedEndPoint;
	long reqport = 0;
	long retport = 0;
	if ( pport )
		{
		reqport = *pport;
		}
	long qlen = 0;
	if ( reqport )
		{
		qlen = 1;
		}
	
	OSStatus result = noErr;
	// if ( noErr == result )
		{
		OTConfiguration * config = ::OTCreateConfiguration( path );
		ep = ::OTOpenEndpoint( config, 0, nil, &result );
		}
	if ( noErr == result )
		{
		::OTInitInetAddress( &reqaddr, reqport, 0 );
		reqbind.addr.buf    = (uchar *) &reqaddr;
		reqbind.addr.len    = sizeof(reqaddr);
		reqbind.addr.maxlen = sizeof(reqaddr);
		reqbind.qlen        = qlen;
		
		::OTInitInetAddress( &retaddr, 0, 0 );
		retbind.addr.buf    = (uchar *) &retaddr;
		retbind.addr.len    = 0;		// was sizeof(retaddr);
		retbind.addr.maxlen = sizeof(retaddr);
		retbind.qlen        = 0;
		
		result = ::OTBind( ep, &reqbind, &retbind );
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
**	DTSNetClose()		+++	Unix Sockets +++
**
**	close an endpoint
*/
void
DTSNetClose( DTSEndPoint ep )
{
	if (mSocket != INVALID_SOCKET)
		close(mSocket);
	
	if ( IsEndPointOpen(ep) )
		{
		::OTCloseProvider( ep );
		}
}


/*
**	DTSNetAlloc()		+++	Unix Sockets +++
**
**	allocate a listen call structure
*/
DTSError
DTSNetAlloc( DTSEndPoint ep, DTSNetCall ** pcall )
{
	DTSNetCall * call = nil;
	
	OSStatus result = noErr;
	// if ( noErr == result )
		{
		call = (DTSNetCall *) ::OTAlloc( ep, T_CALL, T_ALL, &result );
		}
	
	*pcall = call;
	return result;
}


/*
**	DTSNetFree()		+++	Unix Sockets +++
**
**	free a listen call structure
*/
void
DTSNetFree( DTSNetCall * call )
{
	if ( call )
		{
		::OTFree( call, T_CALL );
		}
}


/*
**	DTSNetListen()		+++	Unix Sockets +++
**
**	listen for new connections
*/
DTSError
DTSNetListen( DTSEndPoint ep, DTSNetCall * call )
{
	// we have no conditions
	call->addr.len  = 0;
	call->opt.len   = 0;
	call->udata.len = 0;
	call->sequence  = 0;
	
	DTSError result = noErr;
	// if ( noErr == result )
		{
		result = ::OTListen( ep, call );
		}
	
	// convert no data error
	if ( kOTNoDataErr == result )
		{
		result = 1;
		}
	
	return result;
}


/*
**	DTSNetAccept()		+++	Unix Sockets +++
**
**	accept new connections
*/
DTSError
DTSNetAccept( DTSEndPoint hostep, DTSEndPoint chanep, DTSNetCall * call )
{
	DTSError result = noErr;
	// if ( noErr == result )
		{
		result = ::OTAccept( hostep, chanep, call );
		}
	
	// handle look errors
	if ( kOTLookErr == result )
		{
		result = ::OTLook( hostep );
		switch ( result )
			{
			// another listen event
			// will be handled by caller
			case T_LISTEN:
				result = kNetworkListen;
				break;
			
			// we've been disconnected
			case T_DISCONNECT:
				::OTRcvDisconnect( hostep, nil );
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
**	DTSNetSend()		+++	Unix Sockets +++
**
**	send data
*/
DTSError
DTSNetSend( DTSEndPoint ep, void * data, long size )
{
	// send the data
	OTResult result = ::OTSnd( ep, data, size, 0 );
	
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
		result = ::OTLook( ep );
		switch ( result )
			{
			// we've been disconnected
			case T_DISCONNECT:
				::OTRcvDisconnect( ep, nil );
				result = kNetworkDisconnect;
				break;
			
			// we've been disconnected
			case T_ORDREL:
				::OTRcvOrderlyDisconnect( ep );
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
**	DTSNetReceive()		+++	Unix Sockets +++
**
**	receive data
*/
DTSError
DTSNetReceive( DTSEndPoint ep, void * data, long * size )
{
	long numread = 0;
	OTFlags flags;
	OTResult result = ::OTRcv( ep, data, *size, &flags );
	
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
		result = ::OTLook( ep );
		switch ( result )
			{
			// we've been disconnected
			case T_DISCONNECT:
				::OTRcvDisconnect( ep, nil );
				result = kNetworkDisconnect;
				break;
			
			// we've been disconnected
			case T_ORDREL:
				::OTRcvOrderlyDisconnect( ep );
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
**	DTSNetUSend()		+++	Unix Sockets +++
**
**	send udp data
*/
DTSError
DTSNetUSend( DTSEndPoint ep, uchar * toaddr, void * data, long size )
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
	OSStatus result = ::OTSndUData( ep, &ud );
	
	// handle look errors
	if ( kOTLookErr == result )
		{
		result = ::OTLook( ep );
		switch ( result )
			{
			// unit data error
			case T_UDERR:
				::OTRcvUDErr( ep, nil );
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
**	DTSNetUReceive()		+++	Unix Sockets +++
**
**	send udp data
*/
DTSError
DTSNetUReceive( DTSEndPoint ep, uchar * wantaddr, void * data, long * size )
{
	// separate the address we get from t_rcvudata
	// and the address the caller wants to receive data from
	uchar fromaddr[16];
	std::memcpy( fromaddr, wantaddr, sizeof(fromaddr) );
	
	// initialize the structure
	TUnitData ud;
	uchar optbuf[600];
	ud.addr.maxlen  = sizeof(fromaddr);
	ud.addr.len     = 0;
	ud.addr.buf     = fromaddr;
	ud.opt.maxlen   = sizeof(optbuf);
	ud.opt.len      = 0;
	ud.opt.buf      = optbuf;
	long numread    = *size;
	ud.udata.maxlen = numread;
	ud.udata.len    = 0;
	ud.udata.buf    = (uchar *) data;
	
	// read data
	OTFlags flags;
	numread = 0;
	OSStatus result = ::OTRcvUData( ep, &ud, &flags );
	
	// a good receive result
	if ( noErr == result )
		{
		// make sure we got the data from where we wanted to
		// a good result
		if ( wantaddr[1] != 2
		||   std::memcmp( fromaddr, wantaddr, sizeof(fromaddr) ) == 0 )
			{
			numread = ud.udata.len;
			
			// save the address if we need to
			if ( wantaddr[1] == 0 )
				{
				std::memcpy( wantaddr, fromaddr, sizeof(fromaddr) );
				}
			}
#if DEBUG_VERSION && TARGET_API_MAC_CARBON
	// work around what seems to be a bug in the OpenTransport
	// compatibility layer of OS X (Cf. Radar #2778298)
	// ideally this branch would be ignored on Carbon+OS8/9
	// but wotthehell, right? In principle, OT should be
	// making sure that we DO "get the data from where we wanted to"
	// else how did OTRcvUData() return noErr?

	// On the other hand, let's restrict this "fix" to the debug
	// version for the time being, just to play it safe.
	
		else
			numread = ud.udata.len;
#endif
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
		result = ::OTLook( ep );
		switch ( result )
			{
			// unit data error
			case T_UDERR:
				::OTRcvUDErr( ep, nil );
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
**	DTSNetConnect()		+++	Unix Sockets +++
**
**	connect to a host
*/
DTSError
DTSNetConnect( DTSEndPoint ep, uchar * hostaddr, DTSNetChannel * channel )
{
	DTSNetCall reqcall;
	volatile long notifierresult;
	DTSBoolean bNotifierInstalled = false;
	DTSBoolean bSetAsynchronous = false;
	
	DTSError result = noErr;
	// if ( noErr == result )
		{
		notifierresult = 1;
#if TARGET_API_MAC_CARBON
		result = ::OTInstallNotifier( ep, MakeUPP(MyNotifier,OTNotify), (void *) &notifierresult );
#else
		result = ::OTInstallNotifier( ep, MyNotifier, (void *) &notifierresult );
#endif
		}
	if ( noErr == result )
		{
		bNotifierInstalled = true;
		result = ::OTSetAsynchronous( ep );
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
		
		result = ::OTConnect( ep, &reqcall, nil );
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
		result = ::OTSetSynchronous( ep );
		}
	if ( bNotifierInstalled )
		{
		::OTRemoveNotifier( ep );
		}
	
	if ( noErr == result )
		{
		result = ::OTRcvConnect( ep, nil );
		}
	if ( kNetworkDisconnect == result )
		{
		::OTRcvDisconnect( ep, nil );
		}
	
	// handle look errors
	if ( kOTLookErr == result )
		{
		result = ::OTLook( ep );
		switch ( result )
			{
			// we've been disconnected
			case T_DISCONNECT:
				::OTRcvDisconnect( ep, nil );
				result = kNetworkDisconnect;
				break;
			
			// we've been disconnected
			case T_ORDREL:
				::OTRcvOrderlyDisconnect( ep );
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
**	DTSNetResolveName()		+++	Unix Sockets +++
**
**	resolve the domain name to an ip address
*/
DTSError
DTSNetResolveName( DTSEndPoint ep, char * str, uchar * addr )
{
	DNSAddress dnsAddr;
	TBind reqAddr;
	reqAddr.addr.buf    = (uchar *) &dnsAddr;
	reqAddr.addr.len    = ::OTInitDNSAddress( &dnsAddr, str );
	reqAddr.addr.maxlen = sizeof(dnsAddr);
	reqAddr.qlen        = 0;
	
	TBind retAddr;
	retAddr.addr.buf    = addr;
	retAddr.addr.len    = 0;
	retAddr.addr.maxlen = 16;	// or sizeof(InetAddress) ??
	retAddr.qlen        = 0;
	
	// to be really fancy, we'd do this asynchronously too...
	OSStatus result = ::OTResolveAddress( ep, &reqAddr, &retAddr, TIMEOUT );
	
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
	OSStatus err;

#if TARGET_API_MAC_CARBON

	// what could be easier? <g>
	InetInterfaceInfo info;
	err = ::OTInetGetInterfaceInfo( &info, kDefaultInetInterface );
	if ( noErr == err )
		std::memcpy( buff, info.fHWAddr, info.fHWAddrLen );
	return err;

#else

	// first, attempt to get the burned-in ethernet address 
	// directly from ROM or NameRegistry

	UInt8 enetaddr[6] = {0};
	UInt32 cputype = DoesCPUHaveBuiltInEthernet();
	
	switch (cputype)
		{
		case kPDMMachine:
			err = GetPDMBuiltInEnetAddr(enetaddr);
			if (noErr == err)
				{
				std::memcpy( buff, enetaddr, sizeof(enetaddr) );
				return noErr;
				}
			break;
		
		case kPCIMachine:
			err = GetPCIBuiltInEnetAddr(enetaddr);
			if (noErr == err)
				{
				std::memcpy( buff, enetaddr, sizeof(enetaddr) );
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
				std::memcpy( buff, enetaddr, sizeof(enetaddr) );
				return noErr;
				}
			break;
		
		default:
			// It appears that this CPU does not have built-in ethernet
			break;
		}
	
	// that didn't work
	// ask open transport
	T8022Address theAddr       = {AF_8022, {0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 0x8888, {0x00,0x00,0x00,0x00,0x00}};
	T8022Address theReturnAddr = {AF_8022, {0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 0x8888, {0x00,0x00,0x00,0x00,0x00}};
	
	for ( long index = 0;  ;  ++index )
		{
		OTPortRecord devicePortRecord;
		DTSBoolean foundAPort = ::OTGetIndexedPort( &devicePortRecord, index );
		if ( foundAPort == false )
			return -1;
		
		if ( ( devicePortRecord.fCapabilities & kOTPortIsDLPI ) == 0 )
			continue;
		if ( ( devicePortRecord.fCapabilities & kOTPortIsTPI ) == 0 )
			continue;
		
		long deviceType = ::OTGetDeviceTypeFromPortRef( devicePortRecord.fRef );
		if ( deviceType != kOTEthernetDevice )
			continue;
		
		OTConfiguration * config = ::OTCreateConfiguration( devicePortRecord.fPortName );
		if ( config == nil )
			continue;
		
		OSStatus result;
		EndpointRef ep = ::OTOpenEndpoint( config, (OTOpenFlags) 0, nil, &result );
		if ( result != noErr )
			continue;
		
		TBind requestInfo;
		requestInfo.addr.buf    = (UInt8 *) &theAddr;
		requestInfo.addr.len    = 10;
		requestInfo.addr.maxlen = 0;			
		requestInfo.qlen        = 0;
		result = ::OTBind( ep, &requestInfo, nil );
		
		if ( noErr == result )
			{
			TBind returnInfo;
			returnInfo.addr.buf    = (UInt8 *) &theReturnAddr;
			returnInfo.addr.maxlen = sizeof(theReturnAddr);
			returnInfo.qlen        = 0;
			
			result = ::OTGetProtAddress( ep, &returnInfo, nil );
			
			::OTUnbind( ep );
			}
		
		::OTCloseProvider( ep );
		
		if ( result != noErr )
			continue;
		
		std::memcpy( buff, theReturnAddr.fHWAddr, 6 );
		return noErr;
		}
#endif		// ! TARGET_API_MAC_CARBON
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
	
	err = ::Gestalt(gestaltMachineType, &response);
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
		err = ::Gestalt( gestaltNameRegistryVersion, &response );
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
	RegEntryIter			cookie;
	RegEntryID				theFoundEntry;
	char					enetAddrStr[32] = "local_mac-address";
	RegCStrEntryNamePtr		enetAddrCStr = enetAddrStr;
	RegEntryIterationOp		iterOp = kRegIterDescendants;
	UInt8					enetAddr[6] = { 0 };
	DTSBoolean				done = false;

	long data;
	err = ::Gestalt( gestaltNameRegistryVersion, &data );
	if ( err != noErr )
		return err;

	err = ::RegistryEntryIDInit( &theFoundEntry );
	if (err != noErr)
		return err;

	err = ::RegistryEntryIterateCreate( &cookie );
	if (err != noErr)
		return err;

	err = ::RegistryEntrySearch( &cookie, iterOp, &theFoundEntry, &done,
	                                                  enetAddrCStr, nil, 0);

	if (noErr == err)
		{
	    RegPropertyValueSize theSize = sizeof(enetAddr);
	    err = ::RegistryPropertyGet( &theFoundEntry, enetAddrCStr, &enetAddr, &theSize );
	    if (noErr == err)
	    	::BlockMoveData( enetAddr, enetaddr, sizeof(enetAddr) );
	}

	::RegistryEntryIterateDispose( &cookie );

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
    RegEntryIterationOp     iterOp = kRegIterDescendants;
    UInt8					*enetAddr = 0;
    DTSBoolean              done = false;
	
	long data;
	err = ::Gestalt( gestaltNameRegistryVersion, &data );
	if ( err != noErr )
		return err;
	
    err = ::RegistryEntryIDInit( &theFoundEntry );
	if ( err != noErr )
		{
//		fprintf(stdout, "RegistryEntryIDInit failed\n");
		return err;
		}

	err = ::RegistryEntryIterateCreate( &cookie );
	if ( err != noErr )
		{
//		fprintf(stdout, "RegistryEntryIterateCreate failed\n");
		return err;
		}

	err = ::RegistryEntrySearch( &cookie, iterOp, &theFoundEntry, &done,
	                                                  enetAddrCStr, nil, 0 );

	if ( noErr == err )
		{
		RegPropertyValueSize theSize = sizeof(enetAddr);
		err = ::RegistryPropertyGet( &theFoundEntry, enetAddrCStr, &enetAddr, &theSize );
		if ( noErr == err )
			::BlockMoveData( enetAddr, enetaddr, 6 );
		}

	::RegistryEntryIterateDispose( &cookie );

	return err;
#endif	// 68K
}

#endif // !TARGET_API_MAC_CARBON


