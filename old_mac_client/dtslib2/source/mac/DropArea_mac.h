/*	DropArea_mac.h		dtslib2
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

// (a lot of this code was lifted from PowerPlant.)

#ifndef DROPAREA_MAC_H
#define DROPAREA_MAC_H

// controls whether we use the new (Pasteboard-based) or old Drag APIs [new code isn't quite done]
#define DRAGS_WITH_PASTEBOARD		0


/*
 *	All the drop areas in an application are stored in a linked
 *	list, so that they can be managed by a single pair of Drag Manager
 *	callbacks functions, instead of having to install separate
 *	drag-tracking and drag-receiver functions into every window.
 *	This is an implementation detail.
 */
struct DropAreaNode;


/*
 *	DropArea is an abstract base class that represents a potential
 *	drop target, for drag-and-drop operations.
 */
class DropArea
{
public:

	//
	// constructor/destructor
	//
	
							// constructor -- the window parameter is necessary,
							// since all drop targets are inside a window
	explicit			DropArea( WindowRef inMacWindow );
	
							// destructor
	virtual				~DropArea();
	
private:
		// declared but not defined, to prevent unwanted copying
						DropArea( const DropArea& rhs );
	DropArea&			operator=( const DropArea& );
	
public:
	//
	// Drag Tracking
	//
							/*
							 * Pure virtual function that tests whether the current
							 * drag is inside a drop area. Your subclass(es) MUST
							 * override this to provide an implementation.
							 */
	virtual bool		IsPointInside( const DTSPoint& inPoint ) const = 0;
	

protected:
	
	//
	//	data members
	//
	
	WindowRef		mWindow;			// the window containing this drop area
	bool			mDragAcceptable;	// whether the drop area can accept the drag
	bool			mHilited;			// whether drop hiliting has been applied
	
	
	//
	// Drag Hiliting
	//
							/*
							 * Called when it's necessary to apply drop hiliting
							 * to a drop area.
							 */
	virtual void		HiliteDropArea( DragRef inDragRef );
	
							/*
							 * Called when the drop hiliting must be removed.
							 */
	virtual void		UnhiliteDropArea( DragRef inDragRef );
	
	//
	// Drag Flavor Taste-testing
	//
	
							/*
							 * Determines whether the contents of a drag are
							 * acceptable to the current drop area. You can
							 * override this if necessary, but it may not be
							 * necessary: the default method calls IsDragItemAcceptable()
							 * for each item in the drag and consolidates the
							 * answers.
							 */
	virtual bool		IsDragAcceptable( DragRef inDragRef ) const;
	
							/*
							 * Determine whether a specific drag -item- is
							 * acceptable. You should definitely override this, if
							 * you don't override the previous method.
							 */
#if DRAGS_WITH_PASTEBOARD
	virtual bool		IsDragItemAcceptable( PasteboardRef inPB, PasteboardItemID iid ) const;
#else
	virtual bool		IsDragItemAcceptable( DragRef inDragRef, DragItemRef inItemRef ) const;
#endif
	
	//
	// Drag Tracking
	//
							/*
							 * Enter() and Leave() are each called once, at the
							 * appropriate time, as the drag enters and leaves
							 * the drop area. It's not normally necessary to
							 * override these.
							 */
	virtual void		EnterDropArea( DragRef inDragRef, bool hasLeftSender );
	virtual void		LeaveDropArea( DragRef inDragRef );
	
							/*
							 * Inside() is called repeatedly while the drag
							 * lingers inside a single drop area. Again, the
							 * default implementation is probably perfectly
							 * fine for most purposes.
							 */
	virtual void		InsideDropArea( DragRef inDragRef );
	
	//
	// Drop Handling (receiving)
	//
	
							/*
							 * This routine is called when a drag is received.
							 * You can override it if you want to deal with the
							 * entire drag payload as a single entity; otherwise,
							 * just use the default method, which breaks the drop
							 * up into individual items and calls the next routine
							 * for each one.
							 */
	virtual void		DoDragReceive( DragRef inDragRef );
	
							/*
							 * The default version of the previous function
							 * iterates over all items in the drag, and calls this
							 * function repeatedly for each one. You're required to
							 * provide your own implementation, unless you've replaced
							 * DoDragReceive() above (in which case this method might
							 * nevertheless still be a useful way to factorize the
							 * handling of individual drag items).
							 *
							 * These would normally be pure-virtual, to force subclasses to
							 * implement them -- except that if a subclass wholly reimplements
							 * DoDragReceive() instead, it may not need to provide these too.
							 */
#if DRAGS_WITH_PASTEBOARD
	virtual void		ReceiveDragItem( DragRef inDrag,
								PasteboardRef inPBRef,
								DragAttributes inDragAttrs,
								PasteboardItemID inItemID );
#else
	virtual void		ReceiveDragItem( DragRef inDragRef,
								DragAttributes inDragAttrs,
								DragItemRef inItemRef );
#endif
	
	//
	// Class Data
	//
	
								/*
								 * The linked list of all drop areas
								 */
	static DropAreaNode *	gRootDropArea;
	
								/*
								 * Which drop area is currently active
								 */
	static DropArea *		sCurrentDropArea;
	
								/*
								 * Whether the drag has left its source window. The Mac
								 * HI Guidelines for drag & drop call for some variance
								 * in drop-area hiliting depending on this flag, so we
								 * make it conveniently accessible here.
								 */
	static bool				sDragHasLeftSender;
	
	
	//
	// Internal machinery -- you normally wouldn't need to touch anything
	// below here.
	//
	
							/*
							 * Add a new drop area to the global list. Called by
							 * the DropArea constructor. New areas are pushed onto
							 * the front of the list, which means that if you have,
							 * say, two areas, one of which is nested inside the
							 * other, you'll want to construct them in order of
							 * increasing depth. That's generally the default anyway,
							 * assuming that the inner one is a "child" of the outer.
							 */
	static void			AddDropArea( DropArea * inDropArea, WindowRef inMacWindow );
	
							/*
							 * Remove a drop area from the global list.
							 * Called by the DropArea destructor.
							 */
	static void			RemoveDropArea( DropArea * inDropArea, WindowRef inMacWindow );
	
							/*
							 * Search the list of drop areas to find which one (if any)
							 * is a candidate for the current drag.
							 */
	static DropArea *	FindDropArea( WindowRef				inMacWindow,
										const DTSPoint&		inPoint,
										DragRef				inDragRef );
	
	
							/*
							 * Called repeatedly by the Drag Manager while a drag is
							 * inside a particular window. This routine then
							 * manages the dispatching of tracking calls amongst the
							 * window's DropAreas.
							 */
	static void			InTrackingWindow( WindowRef inMacWindow, DragRef inDragRef );
	
	
	//
	// Toolbox glue code - these are the callbacks installed into the
	// Drag Manager for each window that contains a DropArea. They provide
	// relatively few services on their own, other than creating a "firewall"
	// for C++ exceptions, and interfacing between the plain C Drag Manager
	// and the object-oriented routines above.
	//
	
	static OSErr		HandleDragTracking(
								DragTrackingMessage		inMessage,
								WindowRef				inMacWindow,
								void * 					ud,
								DragRef					inDragRef );
	
	static OSErr		HandleDragReceive(
								WindowRef				inMacWindow,
								void *					ud,
								DragRef					inDragRef );
};


#pragma mark - class Drop Window -



/*
**	Class DropWindow is a specialization of DropArea, which deals with DTSWindows
**	instead of Mac WindowRefs. Can be used as-is, or you can further subclass this
**	to handle drop targets that are smaller than a whole window.
*/
class DropWindow : public DropArea
{
public:
	
	//
	//	Constructor/destructor -- nothing noteworthy here
	//
						DropWindow( DTSWindow * inWindow );
	virtual				~DropWindow();
	
public:
	//
	// Drag Tracking
	//
	
							/*
							 * The default function tests whether the point
							 * is anywhere inside the window's content region.
							 * Override this if you need better aim.
							 */
	virtual bool		IsPointInside( const DTSPoint& inPoint ) const;
	
	
							/*
							 * The hiliting function applies the standard
							 * Drag Manager tinge to the outer edge of the window.
							 * You might need something smaller.
							 */
	virtual void		HiliteDropArea( DragRef inDragRef );
	
							/*
							 * You may be able to use this function unchanged,
							 * since it simply calls the Drag Manager to remove
							 * any hiliting, regardless of shape & size.
							 * However, if your hiliting function has done anything more
							 * complicated than just passing a custom region to
							 * the Drag Manager's ShowDragHilite() function,
							 * then you'll need to undo that here.
							 */
	virtual void		UnhiliteDropArea( DragRef inDragRef );

private:	// no copying allowed
						DropWindow( const DropWindow& rhs );
	DropWindow&			operator=( const DropWindow& rhs );
};


#pragma mark - UPP Wrapper classes -

//
//	These are stack-based classes for allocating and disposing of Drag Manager UPPs,
//	thereby freeing the main classes from handling those chores. The constructor
//	for the DropArea class allocates one single static instance of each when it runs,
//	so no UPPs are allocated until necessary. The deallocation happens when
//	the program terminates, as part of unravelling the static destructor chain.


/*
**	class StDragTrackingUPP
**	an auto-destructing UPP for drag tracking.
**	The default implementation doesn't use the refcon mechanism, so it remains
**	available to you if you need it. (However, there's no access to such refcons
**	from the overarching DropArea/DropWindow classes, nor from their subclasses,
**	so there's not much point to providing any here.)
*/
class StDragTrackingUPP
{
public:
						StDragTrackingUPP(
							DragTrackingHandlerProcPtr	inProcPtr,
							WindowRef					inMacWindow,
							void *						inUserData = nullptr );
						~StDragTrackingUPP();
						
						operator DragTrackingHandlerUPP() const	{ return mTrackingUPP; }
	
protected:
	DragTrackingHandlerUPP		mTrackingUPP;		// the UPP
	WindowRef					mWindow;			// the window it was installed into
													// (so we can de-install it upon destruction)

private:	// no copying allowed
						StDragTrackingUPP( const StDragTrackingUPP& rhs );
	StDragTrackingUPP&	operator=( const StDragTrackingUPP& rhs );
};


/*
**	class StDragReceiveUPP
**	auto-destructing UPP for drag receiving; identical design to the above.
*/
class StDragReceiveUPP
{
public:
						StDragReceiveUPP(
								DragReceiveHandlerProcPtr	inProcPtr,
								WindowRef					inWindow,
								void *						inUserData = nullptr );
						
						~StDragReceiveUPP();
						
						operator DragReceiveHandlerUPP() const { return mReceiveUPP; }

protected:
	DragReceiveHandlerUPP	mReceiveUPP;			// UPP
	WindowRef				mWindow;				// installed window

private:	// no copying allowed
						StDragReceiveUPP( const StDragReceiveUPP& rhs );
	StDragReceiveUPP&	operator=( const StDragReceiveUPP& rhs );
};


#pragma mark - Useful Utilities -


/*
 *				Drag & Drop Utilities
 *
 * These are various convenience functions that might come in handy while dealing
 * with drags. Feel free to add more. To begin with, I've only put accessors for
 * extracting text and HFS flavors from a drag.
 */


namespace UDragging
{
	// does the given drag item contain a particular data flavor?
	// For the Pasteboard API, "flavors" refer to UTIs -- versus OSTypes for the old API.
#if DRAGS_WITH_PASTEBOARD
	bool			DragItemContainsFlavor(
						   PasteboardRef	inPBRef,
						   PasteboardItemID	inItemID,
						   CFStringRef		inUTType );
#else
	bool			DragItemContainsFlavor(
							DragRef			inDragRef,
							DragItemRef		inItemRef,
							OSType			inFlavor );
#endif  // DRAGS_WITH_PASTEBOARD
	
	// old-fashioned HFSFlavor items are deprecated in the newer pasteboard API.
	// use CopyDragURL instead.
#if ! DRAGS_WITH_PASTEBOARD
	// try to extract an HFSFlavor from a drag. Make sure that the FSSpec
	// didn't get truncated.
	OSStatus		GetHFSFlavor(
							DragRef			inDragRef,
							DragItemRef		inItemRef,
							HFSFlavor &		outHFS );
#endif
	
	// extract a DTSFileSpec from a drag item, if there is one
	// (via a typeFileURL or HFSFlavor)
#if DRAGS_WITH_PASTEBOARD
	OSStatus		GetDragFile(
							PasteboardRef		inPBRef,
							PasteboardItemID	inItemID,
							DTSFileSpec&		outSpec );
#else
	OSStatus		GetDragFile(
							DragRef			inDragRef,
							DragItemRef		inItemRef,
							DTSFileSpec &	outSpec );
#endif  // DRAGS_WITH_PASTEBOARD
	
	// extract a URL from a drag item, if any.
#if DRAGS_WITH_PASTEBOARD
	OSStatus		CopyDragURL(
						PasteboardRef		inPB,
						PasteboardItemID	inItemID,
						CFURLRef&			outURL );
#else
	OSStatus		CopyDragURL(
							DragRef			inDragRef,
							DragItemRef		inItemRef,
							CFURLRef &		outURL );
#endif	// DRAGS_WITH_PASTEBOARD
	
	// extract a text flavor (a zero-terminated string of 8-bit bytes, probably
	// in MacRoman encoding, and labeled as 'TEXT') from a drag.
#if DRAGS_WITH_PASTEBOARD
	OSStatus		GetDragText(
							PasteboardRef		inDragRef,
							PasteboardItemID	inItemRef,
							size_t				inBufferSize,
							char *				outBuffer,	// can be NULL
							size_t *			outActualSize = nullptr );
#else
	OSStatus		GetDragText(
							DragRef			inDragRef,
							DragItemRef		inItemRef,
							size_t			inBufferSize,
							char *			outBuffer,	// can be NULL
							size_t *		outActualSize = nullptr );
#endif
	
	// FIXME: provide routines to extract other kinds of text: UTF8, CFString...
}

#endif	// DROPAREA_MAC_H

