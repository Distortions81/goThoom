/*	DropArea_mac.cp		dtslib2
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

// (Choirs of angels notwithstanding, parts of this code
//  were actually, uh, "borrowed" from PowerPlant.)
//	And besides, it all needs to be redone in terms of Pasteboards and CFURLs and CFStrings.

#include "DropArea_mac.h"

/*
**	Internal Definitions
*/
struct DropAreaNode : DTSLinkedList<DropAreaNode>
{
	DropArea *	theDropArea;
	WindowRef	theWindow;
	
	// ctor dtor
	DropAreaNode( DropArea * inDA, WindowRef inWindow )
		:
		theDropArea( inDA ),
		theWindow( inWindow )
		{
		}
};


/*
**	Class Static Variables
*/
DropAreaNode *	DropArea::gRootDropArea;
DropArea *		DropArea::sCurrentDropArea;
bool			DropArea::sDragHasLeftSender;


	/*
	 *	Class DropArea -- the base class for receiving drags
	 */


/*
**	DropArea::DropArea()
**
**	constructor
**	Installs UPPs for callbacks, and links this drop area into the global list
*/
DropArea::DropArea( WindowRef inMacWindow ) :
	mWindow( inMacWindow ),
	mDragAcceptable(),
	mHilited()
{
	if ( not gRootDropArea )
		{
		// lazy instantiation of the Drag Manager callback UPPs;
		// only happens when the first drop area is created (if ever).
		// By specifying a WindowRef of 0 (aka kLastWindowOfClass), we're saying
		// "use these callbacks for every window in the application"
		
		static StDragTrackingUPP sDragTrackingUPP( HandleDragTracking, kLastWindowOfClass );
		static StDragReceiveUPP	 sDragReceiveUPP( HandleDragReceive, kLastWindowOfClass );
		}
	
	// link this area into the global chain
	AddDropArea( this, inMacWindow );
}


/*
**	DropArea::~DropArea()
**
**	destructor
*/
DropArea::~DropArea()
{
	// remove self from the global chain
	RemoveDropArea( this, mWindow );
}


	//
	//		Drag Hiliting
	//


/*
**	DropArea::HiliteDropArea()
**
**	Apply the appropriate visual drop-destination hiliting feedback for a
**	drop area that's willing and able to accept a drag.
**	Subclasses must override!
*/
void
DropArea::HiliteDropArea( DragRef /* inDragRef */ )
{
	mHilited = false;
}


/*
**	DropArea::UnhiliteDropArea()
**
**	undoes the effect of the foregoing. Default implementation just tells the
**	Drag Manager to remove all hiliting that it has provided at our behest.
*/
void
DropArea::UnhiliteDropArea( DragRef inDragRef )
{
	__Verify_noErr( HideDragHilite( inDragRef ) );
}


	//
	//		Drag Tracking
	//


/*
**	DropArea::EnterDropArea()
**
**	Called when a drag first enters a drop area that can accept it.
*/
void
DropArea::EnterDropArea( DragRef inDragRef, bool hasLeftSender )
{
	// turn on hiliting for the area, but only if the drag has left the
	// sender window -- as per HI Guidelines.
	if ( hasLeftSender )
		{
		mHilited = true;
		HiliteDropArea( inDragRef );
		}
}


/*
**	DropArea::LeaveDropArea()
**
**	Called when a drag leaves a drop area.
*/
void
DropArea::LeaveDropArea( DragRef inDragRef )
{
	// remove any leftover hiliting
	if ( mHilited )
		{
		mHilited = false;
		UnhiliteDropArea( inDragRef );
		}
	
	// dropping is not allowed anymore, since the drag no longer inside a drop area
	mDragAcceptable = false;
}


/*
**	DropArea::InsideDropArea()
**
**	Called repeatedly while a drag remains inside a single drop area.
*/
void
DropArea::InsideDropArea( DragRef /* inDragRef */ )
{
}


	//
	//		Drag Flavor Taste-Testing
	//


/*
**	DropArea::IsDragAcceptable()
**
**	Called to determine if the entire contents of a drag are acceptable.
**	Override if you wish, or just use this version, which iterates over all
**	drag items, calls IsDragItemAcceptable() for each one, and tallies up
**	a consolidated yes/no answer. Remember that acceptance is all or nothing:
**	unless you can accept every single item in a drag, you must reject it.
*/
bool
DropArea::IsDragAcceptable( DragRef inDragRef ) const
{
	// assume it's wanted
	bool wantIt = true;
	
	// count the items in this drag
#if DRAGS_WITH_PASTEBOARD
	PasteboardRef pb = nullptr;
	ItemCount nItems = 0;
	OSStatus err = GetDragPasteboard( inDragRef, &pb );
	__Check_noErr( err );
	
	if ( noErr == err )
		{
		err = PasteboardGetItemCount( pb, &nItems );
		__Check_noErr( err );
		}
#else
	UInt16 nItems( 0 );
	OSStatus err = CountDragItems( inDragRef, &nItems );
	__Check_noErr( err );
#endif
	
	if ( noErr == err )
		{
		// look at each item
		for ( uint ii = 1; ii <= nItems; ++ii )
			{
#if DRAGS_WITH_PASTEBOARD
			PasteboardItemID iid( nullptr );
			err = PasteboardGetItemIdentifier( pb, ii, &iid );
#else
			DragItemRef ref( 0 );
			err = GetDragItemReferenceNumber( inDragRef, ii, &ref );
#endif
			__Check_noErr( err );
			
			// is this item desired?
			if ( noErr == err )
				{
#if DRAGS_WITH_PASTEBOARD
				wantIt = IsDragItemAcceptable( pb, iid );
#else
				wantIt = IsDragItemAcceptable( inDragRef, ref );
#endif
				
				// bail the instant we find an unwanted item, since that
				// means the entire drag is unacceptable.
				// [Right?]
				if ( not wantIt )
					break;
				}
			}
		}
	
	return wantIt;
}


/*
**	DropArea::IsDragItemAcceptable()
**
**	Called to determine if an individual drag -item- is acceptable.
**	Subclasses must override!
*/
#if DRAGS_WITH_PASTEBOARD
bool
DropArea::IsDragItemAcceptable( PasteboardRef /* inPB */, PasteboardItemID /* iid */ ) const
{
	return false;
}
#else
bool
DropArea::IsDragItemAcceptable( DragRef /* inDragRef */, DragItemRef /* inItemRef */ ) const
{
	return false;
}
#endif  // DRAGS_WITH_PASTEBOARD


	//
	//		Drag Receive Handling
	//


/*
**	DropArea::DoDragReceive()
**
**	Receive a drag. This version just iterates over all the drag items
**	and calls ReceiveDragItem() on each one. Neither routine will ever be called
**	unless the drag has passed the IsDragAcceptable() test, so you don't need to
**	re-verify acceptability.
*/
void
DropArea::DoDragReceive( DragRef inDragRef )
{
	// pick up the drag attributes
	DragAttributes dragAttrs;
	OSStatus err = GetDragAttributes( inDragRef, &dragAttrs );
	__Check_noErr( err );
	
	if ( noErr == err )
		{
		// count # of drag items
#if DRAGS_WITH_PASTEBOARD
		PasteboardRef pb( nullptr );
		ItemCount nItems( 0 );
		err = GetDragPasteboard( inDragRef, &pb );
		__Check_noErr( err );
		if ( noErr == err )
			{
			err = PasteboardGetItemCount( pb, &nItems );
			__Check_noErr( err );
			}
#else
		UInt16 nItems( 0 );
		err = CountDragItems( inDragRef, &nItems );
		__Check_noErr( err );
#endif
		
		// loop over each item
		if ( noErr == err )
			{
			for ( uint ii = 1; ii <= nItems; ++ii )
				{
#if DRAGS_WITH_PASTEBOARD
				PasteboardItemID iid;
				err = PasteboardGetItemIdentifier( pb, ii, &iid );
#else
				DragItemRef itemRef;
				err = GetDragItemReferenceNumber( inDragRef, ii, &itemRef );
#endif
				__Check_noErr( err );
				
				// receive this item
				if ( noErr == err )
					{
#if DRAGS_WITH_PASTEBOARD
					ReceiveDragItem( inDragRef, pb, dragAttrs, iid );
#else
					ReceiveDragItem( inDragRef, dragAttrs, itemRef );
#endif
					}
				}
			}
		}
}


/*
**	DropArea::ReceiveDragItem()
**
**	Called to receive a single drag item.
**	Subclasses must override!
*/
#if DRAGS_WITH_PASTEBOARD
void
DropArea::ReceiveDragItem(	DragRef				/* inDragRef */,
							PasteboardRef		/* inPB */,
							DragAttributes		/* inAttributes */,
							PasteboardItemID	/* inItemID */ )
{
}
#else
void
DropArea::ReceiveDragItem(	DragRef			/* inDragRef */,
							DragAttributes	/* inAttributes */,
							DragItemRef		/* inItemRef */ )
{
}
#endif  // DRAGS_WITH_PASTEBOARD

#pragma mark -


	//
	//		Internal machinery, such as class static functions.
	//		There shouldn't be any reason to change these
	//		(except when we replace all of this stuff with CarbonEvent-based dragging support).
	//


/*
**	DropArea::AddDropArea()			[static]
**
**	Add a drop area onto the global linked list. Called by DropArea constructor.
*/
void
DropArea::AddDropArea( DropArea * inDropArea, WindowRef inMacWindow )
{
	DropAreaNode * node = NEW_TAG("DropAreaNode") DropAreaNode( inDropArea, inMacWindow );
	__Check( node );
	if ( node )
		node->DropAreaNode::InstallFirst( gRootDropArea );
}


/*
**	DropArea::RemoveDropArea()		[static]
**
**	Remove a drop area from the linked list; called by the destructor.
*/
void
DropArea::RemoveDropArea( DropArea * inDropArea, WindowRef inMacWindow )
{
	for ( DropAreaNode * test = gRootDropArea;  test;  test = test->linkNext )
		{
		if ( test->theDropArea == inDropArea
		&&	 test->theWindow   == inMacWindow )
			{
			test->Remove( gRootDropArea );
			delete test;
			break;
			}
		}
}


/*
**	DropArea::FindDropArea			[static]
**
**	Search the linked list to find a drop area for the current drag.
**	The search is in reverse chronological order of creation, with the intention
**	that any inner subviews get tested before their enclosing parent views, if
**	such a case arises. For disjoint drop areas, of course, it doesn't matter.
*/
DropArea *
DropArea::FindDropArea( WindowRef inMacWindow, const DTSPoint& inPt, DragRef inDragRef )
{
	// assume none found
	DropArea * area = nullptr;
	
	for ( DropAreaNode * test = gRootDropArea;  test;  test = test->linkNext )
		{
		DropArea * testArea = test->theDropArea;
		
		// requirements:
		// must be the right window,
		// must be inside the drop area,
		// and must be acceptable
		
		if ( test->theWindow == inMacWindow
		&&	 testArea->IsPointInside( inPt )
		&&	 testArea->IsDragAcceptable( inDragRef ) )
			{
			// found it
			area = test->theDropArea;
			break;
			}
		}
	
	return area;
}


/*
**	DropArea::InTrackingWindow()	[static]
**
**	This function is called continuously, so long as a drag remains inside
**	a single window. In turn, it's responsible for locating candidate DropAreas,
**	and sending them the right messages in the right sequence.
*/
void
DropArea::InTrackingWindow( WindowRef inMacWindow, DragRef inDragRef )
{
	// locate the drag position
	DTSPoint globalMouse;
	OSStatus err = GetDragMouse( inDragRef, DTS2Mac(&globalMouse), nullptr );
	__Check_noErr( err );
	if ( noErr == err )
		{
		// see if it's inside a DropArea
		DropArea * area = FindDropArea( inMacWindow, globalMouse, inDragRef );
		
		if ( not area )
			{
			// not inside any area
			if ( sCurrentDropArea )
				{
				// we must have just left this previous area
				sCurrentDropArea->LeaveDropArea( inDragRef );
				}
			
			sCurrentDropArea = nullptr;
			sDragHasLeftSender = true;
			}
		else
			{
			// inside a drop area which can accept this drag
			area->mDragAcceptable = true;
			
			if ( area == sCurrentDropArea )
				{
				// still inside the same one
				area->InsideDropArea( inDragRef );
				}
			else
				{
				// has just moved into a new one
				if ( sCurrentDropArea )
					{
					// leaving an old one first
					sCurrentDropArea->LeaveDropArea( inDragRef );
					sDragHasLeftSender = true;
					}
				
				// inform new area that it's It.
				sCurrentDropArea = area;
				area->EnterDropArea( inDragRef, sDragHasLeftSender );
				area->InsideDropArea( inDragRef );
				}
			}
		}
}


	//
	//		Callback glue routines
	//


/*
**	DropArea::HandleDragTracking()	[static]
**
**	Drag manager callback which is invoked repeatedly while a drag is in progress.
*/
OSErr
DropArea::HandleDragTracking(
	DragTrackingMessage		inMessage,
	WindowRef				inWindow,
	void *					/*ud*/,
	DragRef					inDragRef )
{
	OSStatus err = noErr;
	
	// don't let exceptions filter out!
	try
		{
		// save & setup the grafPort (probably unnecessary)
		StDrawEnv saver( inWindow );
		
		switch ( inMessage )
			{
			// drag has just entered this handler for the first time
			case kDragTrackingEnterHandler:
				{
				// cache the has-left-sender flag
				DragAttributes attr( 0 );
				err = GetDragAttributes( inDragRef, &attr );
				__Check_noErr( err );
				if ( noErr == err )
					sDragHasLeftSender = (attr & kDragHasLeftSenderWindow) != 0;
				}
				break;
			
			case kDragTrackingEnterWindow:
				// drag is entering a window
				// ignore this, since our handler does its own window filtering
				break;
			
			case kDragTrackingInWindow:
				// drag is lingering inside a window
				InTrackingWindow( inWindow, inDragRef );
				break;
			
			case kDragTrackingLeaveWindow:
				// make sure we've informed the current drop area that it's been abandoned.
				if ( sCurrentDropArea )
					{
					sCurrentDropArea->LeaveDropArea( inDragRef );
					sDragHasLeftSender = true;
					}
				sCurrentDropArea = nullptr;
				break;
			
			case kDragTrackingLeaveHandler:
				// drag is leaving handler -- nothing to do.
				break;
			}
		}
	
	// convert exceptions to a catch-all error code, pardon the pun.
	catch (...)
		{
		err = dragNotAcceptedErr;
		}
	
	return err;
}


/*
**	DropArea::HandleDragReceive()	[static]
**
**	Drag manager callback which is invoked to receive a drop.
*/
OSErr
DropArea::HandleDragReceive( WindowRef inMacWindow, void * /*ud*/, DragRef inDragRef )
{
	OSStatus err = noErr;
	
	// keep exceptions inside the library!
	try
		{
		// is there a willing receiver?
		if ( sCurrentDropArea
		&&   sCurrentDropArea->mDragAcceptable )
			{
			StDrawEnv saveEnv( inMacWindow );
			
			// tell receiver to accept the drop
			sCurrentDropArea->DoDragReceive( inDragRef );
			}
		else
			err = dragNotAcceptedErr;
		}
	
	// exceptions are converted to drag rejections
	catch ( ... )
		{
		err = dragNotAcceptedErr;
		}
	
	return err;
}

#pragma mark -


	/*
	 * class DropWindow -- an intermediate drag-area class for DTSWindows
	 */


/*
**	DropWindow::DropWindow()
**
**	constructor, rather trivial
*/
DropWindow::DropWindow( DTSWindow * inWindow ) :
	DropArea( DTS2Mac(inWindow) )
{
}


/*
**	DropWindow::~DropWindow()
**	destructor; even more trivial!
*/
DropWindow::~DropWindow()
{
}


/*
**	DropWindow::PointInDropArea()
**
**	Simply returns true if the point is anywhere inside the window's content region.
**	A subclass would want to provide a more refined answer.
*/
bool
DropWindow::IsPointInside( const DTSPoint& inPoint ) const
{
	WindowPartCode part( 0 );
	WindowRef testWindow( nullptr );
	
#if MAC_OS_X_VERSION_MIN_REQUIRED >= 1050
	HIPoint where = CGPointMake( inPoint.ptH, inPoint.ptV );
	OSStatus err = HIWindowFindAtLocation( &where,
						kHICoordSpace72DPIGlobal, nullptr,
						kNilOptions, &testWindow, &part, nullptr );
	__Check_noErr( err );
#else
	WindowClass testClass ( 0 );
	OSStatus err = GetWindowClass( mWindow, &testClass );
	__Check_noErr( err );
	
	if ( noErr == err )
		{
		err = FindWindowOfClass( DTS2Mac( &inPoint ), testClass, &testWindow, &part );
		__Check_noErr( err );
		}
#endif  // 10.5+
	
	return noErr == err && inContent == part && testWindow == mWindow;
}


/*
**	DropWindow::HiliteDropArea()
**
**	Turn on hilighting for a drop area. This default version draws
**	the standard hilite frame at the window's edges.
*/
void
DropWindow::HiliteDropArea( DragRef inDragRef )
{
#if MAC_OS_X_VERSION_MAX_ALLOWED <= 1060
	// determine window's content rect
	Rect bounds;
	GetWindowPortBounds( mWindow, &bounds );
	if ( RgnHandle rgn = NewRgn() )
		{
		// convert that to a region
		RectRgn( rgn, &bounds );
		
		// ask Drag Manager to provide hiliting for the interior of the region
		__Verify_noErr( ShowDragHilite( inDragRef, rgn, true ) );
		DisposeRgn( rgn );
		}
#else  // 10.7+
	   // ShowDragHilite() is deprecated from 10.5. Subclasses should override.
# pragma unused( inDragRef )
#endif  // 10.7
}


/*
**	DropWindow::UnhiliteDropArea()
**
**	Remove any hilighting
*/
void
DropWindow::UnhiliteDropArea( DragRef inDragRef )
{
	// simply tell Drag Manager to remove its hilighting
	__Verify_noErr( HideDragHilite( inDragRef ) );
}

#pragma mark -


	//
	//		Stack-based UPP management
	//

/*
	NB: under Mach, these classes are probably needlessly complex, since
	most NewXXXUPP() functions there are simple no-ops.  In other words,
	these classes' mUPP variables will be exactly the same as their inProcPtrs.
*/


/*
**	StDragTrackingUPP::StDragTrackingUPP()
**	constructor
**	The refcon/user-data parameter isn't used, so it's available if you need it.
*/
StDragTrackingUPP::StDragTrackingUPP(
	DragTrackingHandlerProcPtr			inProcPtr,
	WindowRef							inWindow,
	void * 								inUserData /* default = nullptr */ )
		:
		mTrackingUPP( inProcPtr ),
		mWindow( inWindow )
{
	if ( inProcPtr )
		__Verify_noErr( InstallTrackingHandler( inProcPtr, inWindow, inUserData ) );
}


/*
**	StDragTrackingUPP::~StDragTrackingUPP()
**	destructor
*/
StDragTrackingUPP::~StDragTrackingUPP()
{
	// uninstall the handler from its window
	if ( mTrackingUPP )
		__Verify_noErr( RemoveTrackingHandler( mTrackingUPP, mWindow ) );
}


/*
**	StDragReceiveUPP::StDragReceiveUPP()
**	constructor
**	Same principles as the drag-tracking class
*/
StDragReceiveUPP::StDragReceiveUPP(
	DragReceiveHandlerProcPtr	inProcPtr,
	WindowRef					inMacWindow,
	void *						inUserData /* default = nullptr */ )
		:
		mReceiveUPP( inProcPtr ),
		mWindow( inMacWindow )
{
	if ( inProcPtr )
		__Verify_noErr( InstallReceiveHandler( inProcPtr, inMacWindow, inUserData ) );
}


/*
**	StDragReceiveUPP::~StDragReceiveUPP()
**	destructor
*/
StDragReceiveUPP::~StDragReceiveUPP()
{
	// uninstall the callback from the window
	if ( mReceiveUPP )
		__Verify_noErr( RemoveReceiveHandler( mReceiveUPP, mWindow ) );
}

#pragma mark -


	//
	//		Drag and Drop Utilities & convenience functions
	//

namespace UDragging
{

/*
**	DragItemContainsFlavor()
**
**	does the given drag item contain a particular data flavor?
*/
#if DRAGS_WITH_PASTEBOARD
bool
DragItemContainsFlavor( PasteboardRef inPBRef,
	PasteboardItemID inItemID, CFStringRef inFlavorUTI )
{
	PasteboardFlavorFlags flags;
	OSStatus err = PasteboardGetItemFlavorFlags( inPBRef, inItemID, inFlavorUTI, &flags );
	return noErr == err;
}
#else
bool
DragItemContainsFlavor( DragRef inDragRef, DragItemRef inItemRef, OSType inFlavor )
{
	FlavorFlags flags;
	return noErr == GetFlavorFlags( inDragRef, inItemRef, inFlavor, &flags );
}
#endif  // DRAGS_WITH_PASTEBOARD


#if ! DRAGS_WITH_PASTEBOARD
/*
**	UDragging::GetHFSFlavor()
**
**	a convenience function for getting HFSFlavor data out of a drag item,
**	with a check to make sure all the data is there. (See TechNote 1085).
**	Don't call this unless you already know that an HFSFlavor is indeed
**	present in the drag item.
**
**	** Disabled for the newer API because HFSFlavors are now deprecated. **
*/
OSStatus
GetHFSFlavor( DragRef inDragRef, DragItemRef inItemRef, HFSFlavor& outHFS )
{
	// read the HFSFlavor data into the provided buffer
	Size len = sizeof outHFS;
	OSStatus err = GetFlavorData( inDragRef, inItemRef, kDragFlavorTypeHFS, &outHFS, &len, 0 );
	__Check_noErr( err );
	
	if ( noErr == err )
		{
		// make sure the filename wasn't truncated
		Size minSize = offsetof( HFSFlavor, fileSpec.name )
				+ StrLength( outHFS.fileSpec.name ) + 1;
		
		if ( len < minSize )
			err = cantGetFlavorErr;
		}
	return err;
}
#endif  // ! DRAGS_WITH_PASTEBOARD


/*
**	UDragging::GetDragFile()
**
**	a utility for extracting a DTSFileSpec from a drag item, via URL
*/
#if DRAGS_WITH_PASTEBOARD
OSStatus
GetDragFile( PasteboardRef inPBRef, PasteboardItemID inItemID, DTSFileSpec& outSpec )
{
	// first, check to see if there's any URL data at all
	if ( not DragItemContainsFlavor( inPBRef, inItemID, kUTTypeFileURL ) )
		return badPasteboardFlavorErr;
	
	// get the URL itself
	CFURLRef url = nullptr;
	OSStatus err = CopyDragURL( inPBRef, inItemID, url );
	if ( noErr == err && url )
		{
		// split into parent directory URL and (possible) leafname string
		CFURLRef parent = CFURLCreateCopyDeletingLastPathComponent( kCFAllocatorDefault, url );
		CFStringRef leafname = nullptr;
		if ( not CFURLHasDirectoryPath( url ) )
			leafname = CFURLCopyLastPathComponent( url );
		
		// convert parent URL into an FSRef...
		FSRef parDir;
		if ( parent )
			{
			bool got = CFURLGetFSRef( parent, &parDir );
			if ( not got )
				err = dirNFErr;
//			__Check_noErr( err );
			CFRelease( parent );
			}
		else
			err = dirNFErr;  // ??
		
		// ... and thence into a DTSFileSpec
		if ( noErr == err )
			err = DTSFileSpec_CopyFromFSRef( &outSpec, &parDir );
		
		// now tack on the leafname, if any
		if ( noErr == err && leafname )
			{
			HFSUniStr255 uname;
			err = FSGetHFSUniStrFromString( leafname, &uname );
			__Check_noErr( err );
			
			// bogus!
			if ( noErr == err )
				outSpec.priv.p->SetFileName( &uname );
			}
		
		// cleanup
		if ( leafname )
			CFRelease( leafname );
		}
	
	// cleanup (redux)
	if ( url )
		CFRelease( url );
	
	return err;
}

#else  // ! DRAGS_WITH_PASTEBOARD

OSStatus
GetDragFile( DragRef inDragRef, DragItemRef inItemRef, DTSFileSpec& outSpec )
{
	// first, check to see if there's any HFS data at all
	// TODO: rewrite using URLs, like above version
	if ( not DragItemContainsFlavor( inDragRef, inItemRef, kDragFlavorTypeHFS ) )
		return cantGetFlavorErr;
	
	// go get it, using our paranoid extractor above
	HFSFlavor theFile;
	OSStatus err = GetHFSFlavor( inDragRef, inItemRef, theFile );
	__Check_noErr( err );
	
	if ( noErr == err )
		{
		// now just turn it into a DTSFileSpec
		FSRef ref;
		err = FSpMakeFSRef( &theFile.fileSpec, &ref );
		__Check_noErr( err );
		if ( noErr == err )
			err = DTSFileSpec_CopyFromFSRef( &outSpec, &ref );
		}
	return err;
}
#endif  // DRAGS_WITH_PASTEBOARD


/*
**	UDragging::CopyDragURL()
**
**	Utility for extracting a dragged URL.
**	See technote 2022 on typeFileURL, and the 'furl' drag flavor:
**		<http://developer.apple.com/library/mac/#technotes/tn2022>
**	Conforms to CF "Create/Copy" naming convention; caller must CFRelease() the
**	returned URL reference (if non-NULL).
*/
#if DRAGS_WITH_PASTEBOARD
OSStatus
CopyDragURL( PasteboardRef inPB, PasteboardItemID inItemID, CFURLRef& outURL )
{
	outURL = nullptr;		// paranoia
	
	CFDataRef dataRef = nullptr;
	OSStatus err = PasteboardCopyItemFlavorData( inPB, inItemID, kUTTypeFileURL, &dataRef );
	__Check_noErr( err );
	if ( noErr == err )
		{
		// convert raw data into a real live 100% genuine CFURLRef.
		// The technote says:
		//		"In a nutshell, typeFileURL is a core-foundation URL
		//		 encoded to a stream of bytes in UTF8 format."
		__Check( dataRef );
		if ( dataRef )
			{
			outURL = CFURLCreateWithBytes( kCFAllocatorDefault,
							CFDataGetBytePtr( dataRef ), CFDataGetLength( dataRef ),
							kCFStringEncodingUTF8, nullptr );
			CFRelease( dataRef );
			}
		}
	return err;
}

#else  // ! DRAGS_WITH_PASTEBOARD

OSStatus
CopyDragURL( DragRef inDragRef, DragItemRef inItemRef, CFURLRef& outURL )
{
	outURL = nullptr;		// paranoia
	
	// This name is not in any known CFM headers (or Mach-O ones either)
	const OSType kDragFlavorTypeFileURL = 'furl';
	
	// measure the raw URL data
	CFIndex len;
	uchar * buff = nullptr;
	OSStatus err = GetFlavorDataSize( inDragRef, inItemRef, kDragFlavorTypeFileURL, &len );
	if ( noErr != err )
		return cantGetFlavorErr;	// can't get the data
	
	// allocate memory & fetch the raw data
	if ( noErr == err )
		{
		buff = NEW_TAG( "drag URL" ) uchar[ len ];
		__Check( buff );
		if ( not buff )
			err = memFullErr;
		}
	if ( noErr == err )
		{
		err = GetFlavorData( inDragRef, inItemRef, kDragFlavorTypeFileURL, buff, &len, 0 );
		__Check_noErr( err );
		}
	
	// convert raw data into a real live 100% genuine CFURLRef.
	// The technote says:
	//		"In a nutshell, typeFileURL is a core-foundation URL
	//		 encoded to a stream of bytes in UTF8 format."
	if ( noErr == err )
		outURL = CFURLCreateWithBytes( kCFAllocatorDefault,
					buff, len, kCFStringEncodingUTF8, nullptr );
	else
		outURL = nullptr;	// no luck, pal.
	
	delete[] buff;
	
	return err;
}
#endif	// ! DRAGS_WITH_PASTEBOARD


/*
**	UDragging::GetDragText()
**
**	utility for extracting text from a drag item.
**	"text" means a zero-terminated byte string in this context, i.e., a C string.
*/
#if DRAGS_WITH_PASTEBOARD
OSStatus
GetDragText(
	PasteboardRef		inPBRef,
	PasteboardItemID	inItemID,
	size_t				inMaxSize,
	char *				outBuff,
	size_t *			outActualSize /* = nullptr */ )
{
	// FIXME - this is probably all wrong.  It doesn't consider the text encoding at all;
	// it doesn't transcode back into MacRoman (as our callers would expect);
	// it likely fails with kUTTypePlainText or kUTTypeUTF16ExternalPlainText or ...;
	// What About BOM?
	
	// be pessimistic
	if ( outBuff && inMaxSize > 0 )
		outBuff[ 0 ] = '\0';
	
	// is there any text?
	if ( not DragItemContainsFlavor( inPBRef, inItemID, kUTTypeUTF8PlainText ) )
		return badPasteboardFlavorErr;
	
	CFDataRef dataref;
	OSStatus err = PasteboardCopyItemFlavorData( inPBRef, inItemID,
						kUTTypeUTF8PlainText, &dataref );
	__Check_noErr( err );
	if ( noErr == err )
		{
		if ( dataref )
			{
			size_t len = CFDataGetLength( dataref );
			if ( outActualSize )
				*outActualSize = len;
			if ( outBuff )
				{
				if ( len >= inMaxSize )
					len = inMaxSize - 1;
				CFDataGetBytes( dataref, CFRangeMake( 1, len ), (uchar *) outBuff );
				outBuff[ len ] = 0;
				}
			
			CFRelease( dataref );
			}
		}
	return err;
}

#else  // ! DRAGS_WITH_PASTEBOARD

OSStatus
GetDragText(
	DragRef			inDragRef,
	DragItemRef		inItemRef,
	size_t			inMaxSize,
	char *			outBuff,
	size_t *		outActualSize /* = nullptr */ )
{
	// is there any text?
	if ( not DragItemContainsFlavor( inDragRef, inItemRef, 'TEXT' ) )
		return cantGetFlavorErr;
	
	// Measure it
	Size size;
	OSStatus err = GetFlavorDataSize( inDragRef, inItemRef, 'TEXT', &size );
	__Check_noErr( err );
	
	if ( noErr == err )
		{
		// inform caller of actual data size, if they want to know it
		if ( outActualSize )
			*outActualSize = size;
		
		// now get the data itself, if asked for
		if ( inMaxSize && outBuff != nullptr )
			{
			size = inMaxSize - 1;	// leave room for the trailing null
			err = GetFlavorData( inDragRef, inItemRef, 'TEXT', outBuff, &size, 0 );
			__Check_noErr( err );
			
			// zero-terminate it
			if ( noErr == err )
				outBuff[ size ] = 0;
			else
				outBuff[ 0 ] = 0;
			}
		}
	
	return err;
}
#endif  // ! DRAGS_WITH_PASTEBOARD

} // namespace UDragging

