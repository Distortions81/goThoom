/*
**	AppleEvents_mac.cp		dtslib2
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

#include "Menu_dts.h"
#include "Shell_dts.h"
#include "Utilities_dts.h"
#include "AppleEvent_mac.h"
#include "File_mac.h"
#include "Shell_mac.h"


/*
**	Entry Routines
*/
/*
DTSError	InitAppleEvents();
DTSError	AddAppleEventHandler( DescType, DescType, long, AEEventHandlerUPP );
DTSError	RemoveAppleEventHandler( DescType, DescType, AEEventHandlerUPP );
*/


/*
**	Definitions
*/


/*
**	Internal Routines
*/

static OSErr	HandleOAPP( const AppleEvent *, AppleEvent *, SRefCon );
static OSErr	HandleODOC( const AppleEvent *, AppleEvent *, SRefCon );


/*
**	Internal Variables
*/



/*
**	InitAppleEvents()
**
**	initialize the core apple event handlers
*/
DTSError
InitAppleEvents()
{
	// the 'oapp' handler is also used for 'quit' events.
	// the event's refcon tells the handler what command to send to the application.
	// Likewise for the 'odoc' and 'pdoc' events.
	
	AEEventHandlerUPP oappUPP = NewAEEventHandlerUPP( HandleOAPP );
	AEEventHandlerUPP odocUPP = NewAEEventHandlerUPP( HandleODOC );
	
	DTSError result;
//	if ( noErr == result )
		{
		result = AEInstallEventHandler( kCoreEventClass, kAEOpenApplication,
			oappUPP, mOSNew, false );
		__Check_noErr( result );
		}
	if ( noErr == result )
		{
		result = AEInstallEventHandler( kCoreEventClass, kAEQuitApplication,
			oappUPP, mOSQuit, false );
		__Check_noErr( result );
		}
	if ( noErr == result )
		{
		result = AEInstallEventHandler( kCoreEventClass, kAEOpenDocuments,
			odocUPP, mOSOpen, false );
		__Check_noErr( result );
		}
	if ( noErr == result )
		{
		result = AEInstallEventHandler( kCoreEventClass, kAEPrintDocuments,
			odocUPP, mOSPrint, false );
		__Check_noErr( result );
		}
	return result;
}


/*
**	HandleOAPP()
**
**	Also handles Quit event, depending on refcon being mOSNew or mOSQuit
*/
OSErr
HandleOAPP( const AppleEvent * /*aEvent*/, AppleEvent * /*reply*/, SRefCon refCon )
{
	OSErr result = errAEEventNotHandled;
	if ( DTSMenu * menu = gDTSMenu )
		{
		bool bHandled = menu->DoCmd( refCon );
		if ( bHandled )
			result = noErr;
		}
	
	return result;
}


/*
**	DoOpenDescList()
**
**	helper for HandleODOC()
*/
static OSStatus
DoOpenDescList( const AEDescList * list, long cmd )
{
	long itemsInList;
	OSStatus result = AECountItems( list, &itemsInList );
	__Check_noErr( result );
	
	if ( noErr == result )
		{
		for ( long idx = 1;  idx <= itemsInList;  ++idx )
			{
			FSRef ref;
			result = AEGetNthPtr( list, idx, typeFSRef, nullptr, nullptr,
						&ref, sizeof ref, nullptr );
			__Check_noErr( result );
			if ( result != noErr )
				continue;
			
			result = DTSFileSpec_CopyFromFSRef( &gDTSOpenFile, &ref );
			
			if ( DTSMenu * menu = gDTSMenu )
				(void) menu->DoCmd( cmd );
			
			// if we didn't handle it, tough luck.
			}
		}
	return result;
}


/*
**	WaitReplyProc()
*/
static Boolean
WaitReplyProc( EventRecord * pEvent, SInt32 * sleep, RgnHandle * rgn )
{
	gEvent = *pEvent;
	HandleEvent();
	
	*sleep = 1;
	*rgn   = nullptr;
	
	return false;
}


/*
**	HandleODOC()
**
**	Also handles PDOC, depending on whether the refcon is mOSOpen or mOSPrint
*/
OSErr
HandleODOC( const AppleEvent * aEvent, AppleEvent * /*reply*/, SRefCon refCon )
{
	DTSError result = noErr;
	DTSMenu * menu = gDTSMenu;
	static AEIdleUPP waitReplyUPP;
	
	if ( not menu )
		result = errAEEventNotHandled;
	if ( not waitReplyUPP )
		waitReplyUPP = NewAEIdleUPP( WaitReplyProc );
	if ( noErr == result )
		{
		result = AEInteractWithUser( kNoTimeOut, nullptr, waitReplyUPP );
//		__Check_noErr( result );
		}
	
	if ( noErr == result )
		{
		AEDescList docList = { typeNull, nullptr };
		
		// open direct objects
		result = AEGetParamDesc( aEvent, keyDirectObject, typeAEList, &docList );
		if ( noErr == result )
			result = DoOpenDescList( &docList, refCon );
		AEDisposeDesc( &docList );
		
		// also open redirected documents
		OSErr result2 = AEGetParamDesc( aEvent, keyRedirectedDocumentList, typeAEList, &docList );
		if ( noErr == result2 )
			result = DoOpenDescList( &docList, refCon );
		AEDisposeDesc( &docList );
		}
	
	return static_cast<OSErr>( result );
}


/*
**	AddAppleEventHandler()
**
**	allow app to install a handler of its own
*/
DTSError
AddAppleEventHandler( DescType inClass, DescType eventKey, SRefCon ud, AEEventHandlerUPP handler )
{
	OSStatus result = AEInstallEventHandler( inClass, eventKey, handler, ud, false );
	__Check_noErr( result );
	return result;
}


/*
**	RemoveAppleEventHandler()
**
**	the converse
*/
DTSError
RemoveAppleEventHandler( DescType evtClass, DescType evtID, AEEventHandlerUPP proc )
{
	OSStatus result = AERemoveEventHandler( evtClass, evtID, proc, false );
	__Check_noErr( result );
	return result;
}


