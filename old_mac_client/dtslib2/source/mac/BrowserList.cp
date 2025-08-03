/*
**	BrowserList.cp
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
**
**
**	kind of like a DTSDlgList, but uses a DataBrowser control
**	Note that Dialog_{mac,mach}.cp knows absolutely nothing about these guys!
*/

#include "BrowserList.h"


/*
**	Internal constants
*/

// We stick a custom control-property onto the underlying DB control, using these
// (hopefully unique) identifiers. The _content_ of that property is merely a pointer
// to 'this', the control's supervising BrowserList instance.
// (Historically that pointer would have been stashed in the control's refCon,
// but we don't want to usurp that -- and with control properties, we no longer
// have to.)
//
const UInt32 kOwnedControlTagCreator	= FOUR_CHAR_CODE( 'DTS\306' );
const UInt32 kOwnedControlTagTag		= FOUR_CHAR_CODE( 'Ownr' );


/*
**	Static Data
*/
	// are UPPs needed for the current architecture?
#if BROWSERLIST_USES_UPPS

	// UPPs for static callbacks
DataBrowserItemDataUPP					BrowserList::itemDataCallback;
DataBrowserItemCompareUPP				BrowserList::itemCompareCallback;
//DataBrowserItemNotificationUPP		BrowserList::itemNotificationCallback;
DataBrowserItemNotificationWithItemUPP	BrowserList::itemNotificationWithItemUPP;
DataBrowserItemHelpContentUPP			BrowserList::itemHelpContentCallback;

# if USE_EXTRA_CALLBACKS
DataBrowserAddDragItemUPP				BrowserList::addDragItemCallback;
DataBrowserAcceptDragUPP				BrowserList::acceptDragCallback;
DataBrowserReceiveDragUPP				BrowserList::receiveDragCallback;
DataBrowserPostProcessDragUPP			BrowserList::postProcessDragCallback;

DataBrowserGetContextualMenuUPP			BrowserList::getContextualMenuCallback;
DataBrowserSelectContextualMenuUPP		BrowserList::selectContextualMenuCallback;
# endif	// USE_EXTRA_CALLBACKS

# if USE_CUSTOM_FORMAT
DataBrowserDrawItemUPP					BrowserList::drawItemCallback;
DataBrowserEditItemUPP					BrowserList::editTextCallback;
DataBrowserHitTestUPP					BrowserList::hitTestCallback;
DataBrowserTrackingUPP					BrowserList::trackingCallback;

DataBrowserItemDragRgnUPP				BrowserList::dragRegionCallback;
DataBrowserItemAcceptDragUPP			BrowserList::acceptDragCallback;
DataBrowserItemReceiveDragUPP			BrowserList::receiveDragCallback;
# endif	// USE_CUSTOM_FORMAT

#endif  // BROWSERLIST_USES_UPPS


/*
**	Internal Routines
*/
// get or set the custom control property mentioned above (see kOwnedControlTagCreator),
// so we can easily map a DB HIViewRef back to its supervising BrowserList object.
static OSStatus		SetOwnerTag( const BrowserList *, HIViewRef );
static OSStatus		GetOwnerTag( HIViewRef, BrowserList ** );



/*
**	BrowserList::BrowserList()
**
**	constructor
*/
BrowserList::BrowserList() : view_( nullptr )
{
}


/*
**	BrowserList::~BrowserList()
**
**	destructor
*/
BrowserList::~BrowserList()
{
}


#if 0  // all subclasses are now compositing
/*
**	BrowserList::Init()
**
**	do some setup.
**	(This was originally written for EUS, which uses a modified DTSLib_V1, without
**	the v2 subtleties of DTSImplementFirm etc. So we have to do some unsightly groping-
**	around inside the control's private parts. Sorry.)
*/
OSStatus
BrowserList::Init( DTSView * inParent, HIViewRef inAdoptee )
{

/*
	The way this works is a little different from most DTSLib idioms.
	If you call it with 'inAdoptee' == NULL, then it creates a new Toolbox DataBrowser
	control -- just like this snippet, say:
			DTSControl::Init( kControlCheckBox, "nice title", containerView );
	... would create a brand-new checkbox (CDEF 1 or 23).
	
	So far, not so different. But if inAdoptee != NULL, then we assume it to be a real,
	already-created HIViewRef, and this method then "adopts" it into a DTSControl.
	
	(I didn't want to undertake the semi-monumental task of grafting DB support into
	DTSLib itself, so this was the most expedient course.)
	
	The really modern thing to do would be to use nibs and not have to bother with
	any of this, but that's an even bigger barrel of worms. :(
*/

	if ( not inParent )
		return paramErr;
	
	view_ = inAdoptee;
	
		// make sure we've got an owner window [mildly risky assumption?]
	WindowRef win = GetWindowFromPort( DTS2Mac(inParent) );
	if ( not win )
		return paramErr;
	
	OSStatus err;
//	if ( noErr == err )
		{
		// turn on ControlMgr's automatic drag/drop support
		
		// FIXME: do I need this? I think it may be needed for
		// resizable/movable columns, even if we don't handle drag/drop per se.
		// but then again maybe it'll mess up some other unspoken assumption?
		
		err = SetAutomaticControlDragTrackingEnabledForWindow( win, true );
		__Check_noErr( err );
		}
	
	// Are we "adopting" an existing HIViewRef, or making one from scratch?
	DTSRect box;
	if ( not view_ )
		{
		// No control, so create a new one, in ListView mode.
		// Use a default rectangle (this dstlib1-era interface is mighty crufty!)
		box.Set( 0, 0, 100, 100 );
		err = CreateDataBrowserControl( win, DTS2Mac(&box), kDataBrowserListView, &view_ );
		__Check_noErr( err );
		}
	else
		{
		// Aha, we ARE adopting a extant control. Find its size.
		GetControlBounds( view_, DTS2Mac(&box) );
		
		// Force it into ListView mode. We don't support the other modes at all.
		DataBrowserViewStyle prevStyle;
		err = GetDataBrowserViewStyle( view_, &prevStyle );
		if ( errDataBrowserNotConfigured == err
		||   ( noErr == err && kDataBrowserListView != prevStyle ) )
			{
			err = SetDataBrowserViewStyle( view_, kDataBrowserListView );
			__Check_noErr( err );
			}
		}
	
	// we can't easily "reach inside" the guts of Control_mac.cp, so
	// here we'll attempt to mimic the inner workings of DTSControlPriv::InitPriv()
	if ( noErr == err )
		{
		// thank heavens these are not private!
		DTSViewPriv * vp = DTSView::priv.p;
		DTSControlPriv * p = DTSControl::priv.p;
		
//		p->controlType = 29 * 16;	// CDEF 29 == procID 464
		p->controlHdl = view_;
		vp->viewBounds = box;
		
		// useless but so what.
		DTSControl::SetDeltaArrow( 0 );
		DTSControl::SetDeltaPage( 0 );
		
		Attach( inParent );
		vp->viewVisible = HIViewIsVisible( view_ );	// IsControlVisible( view_ );
		vp->viewContent.Set( &box );
		
		void * ourself = this;
		__Verify_noErr( SetControlProperty( view_,
			kDTSControlOwnerCreator, kDTSControlOwnerTag, sizeof ourself, &ourself ) );
		
		// add our custom inverse-mapping property (pseudo-refcon)
		err = SetOwnerTag( this );
		}
	
	// Turn on the border frame-and-focus indicator
	if ( noErr == err )
		{
		const Boolean wantFrame = true;
		err = SetControlData( view_, kControlEntireControl,
					kControlDataBrowserIncludesFrameAndFocusTag, sizeof wantFrame, &wantFrame );
		__Check_noErr( err );
		}
	
	// turn on scrollbars (maybe premature? Should subclass do this, once it knows more
	// about the quantity of data to be shown?)
	if ( noErr == err )
		err = SetDBHasScrollBars( true, true );
	
	// we have variable-width columns but fixed-height rows
	// (again, maybe subclass should be responsible?)
	if ( noErr == err )
		{
		err = SetDataBrowserTableViewGeometry( view_, true, false );
		__Check_noErr( err );
		}
	
	// only 1 selection at a time please
	if ( noErr == err )
		{
		err = SetDataBrowserSelectionFlags( view_,
						kDataBrowserDragSelect |
						kDataBrowserSelectOnlyOne |
						kDataBrowserResetSelection |
						kDataBrowserNoDisjointSelection |
						kDataBrowserNeverEmptySelectionSet );
		__Check_noErr( err );
		}
	
	// want whole-row hiliting
	if ( noErr == err )
		{
		err = SetDataBrowserTableViewHiliteStyle( view_,
//					kDataBrowserTableViewMinimalHilite
					kDataBrowserTableViewFillHilite
					);
		__Check_noErr( err );
		}
	
	
	// (this setting is ignored on OSX, where databrowsers always have plain background)
# if 0
	// decide how to draw table backgrounds
	if ( noErr == err )
		{
		err = SetDataBrowserListViewUsePlainBackground( view_, false /*true*/ );
		__Check_noErr( err );
		}
# endif  // 0
	
	// enable drag/drop, column reordering, etc. [?]
	if ( noErr == err )
		{
		err = SetControlDragTrackingEnabled( view_, true );
		__Check_noErr( err );
		}
	
	// install the callbacks
	if ( noErr == err )
		err = InstallCallbacks();
	
	// OK, initial setup is pretty much complete.
	// Ask subclass to provide column definitions.
	if ( noErr == err )
		err = AddDBColumns();
	
	// Pretend that the root container was just opened (if "pretend" is the right word...)
	// This should provoke the subclass into supplying some data for the list.
	if ( noErr == err )
		DoContainerOpened( kDataBrowserNoItem, nullptr );
	
	// Ask the control to resize the column widths, as best it can.
	// FIXME: I'm not sure if this call is worth it. The DB's resizing algorithm is pretty
	// simple-minded, and anyway the docs say this call is a no-op when horizontal scroll
	// bars are turned on...
	if ( noErr == err )
		{
		err = AutoSizeDataBrowserListViewColumns( view_ );
		__Check_noErr( err );
		}
	
	return err;
}
#endif  // 0


/*
**	BrowserList::InitFromNib()
**
**	init from an existing DataBrowser, already configured via a nib file
*/
OSStatus
BrowserList::InitFromNib( HIViewRef inView )
{
	__Check( inView );
	if ( not inView )
		return paramErr;
	view_ = inView;
	
	OSStatus err = SetOwnerTag( this, inView );
	
	if ( noErr == err )
		err = InstallCallbacks();
	
	// prepopulate the list
//	if ( noErr == err )
//		DoContainerOpened( kDataBrowserNoItem, nullptr );
	
	if ( noErr == err )
		__Verify_noErr( AutoSizeDataBrowserListViewColumns( inView ) );
	
	return err;
}


/*
**	BrowserList::Exit()
**	mimic DTSDlgList interface. We ourselves do nothing here (except avert crashes)
*/
void
BrowserList::Exit()
{
	// clear out the callbacks, so the OS doesn't attempt to send kDataBrowserItemRemoved
	// and similar notifications to a no-longer-existent subclass instance.
	if ( view_ )
		{
		DataBrowserCallbacks cb;
		cb.version = kDataBrowserLatestCallbacks;
		__Verify_noErr( InitDataBrowserCallbacks( &cb ) );
		__Verify_noErr( SetDataBrowserCallbacks( view_, &cb ) );
		}
}


/*
**	BrowserList::SetSelect()
**	another DTSDlgList relic.
**
**	Thing is, List Mgr lists don't support independently sortable columns, so a row index
**	always identifies the same data (until/unless you manually rebuild the contents).
**	DataBrowsers use ItemIDs to identify _table_ rows (in the backing store),
**	and TableViewRowIndexes to identify _on-screen_ rows, and the two are not necessarily
**	one-to-one.
**	So code updated from DTSDlgLists to BrowserLists should eschew these 2 routines,
**	in favor of Get/SetDBSelectedItem().
**	However, _if_ your list happens to have a simple clean mapping from ItemIDs to
**	row indices, then you can use this as-is.
*/
void
BrowserList::SetSelect( int item )
{
	// remove previous selection
	DataBrowserItemID theID;
	OSStatus err = GetDBSelectedItem( &theID );
	if ( noErr == err && theID > 0 )
		__Verify_noErr( SetDataBrowserSelectedItems( view_, 1, &theID,
							kDataBrowserItemsRemove ) );
	
	// FIXME: this relationship should be generalized, i.e. by a pair of
	// subclass functions such as DataIDForRowIndex() and RowIndexForDataID()
	theID = item + 1;
	(void) SetDBSelectedItem( theID );
	
	// set keyboard focus, while we're at it
	__Verify_noErr( HIViewSetFocus( view_, kHIViewEntireView, kNilOptions ) );
}


/*
**	BrowserList::GetSelect()
**	more DTSDlgList compatibility; see above
*/
int
BrowserList::GetSelect() const
{
	DataBrowserItemID theID;
	OSStatus err = GetDBSelectedItem( &theID );
	if ( noErr == err )
		{
		// FIXME: this relationship needs to be generalized
		return int(theID) - 1;
		}
	
	return -1;
}


// ---------------------------------------------------------------------------
// BrowserList::ItemNotify() -- the databrowser control is trying to tell us
// something which we might need to be aware of. Right now I only care about
// a double-click or a container-opened message.
// ---------------------------------------------------------------------------
void
BrowserList::ItemNotify(
	DataBrowserItemID			item,
	DataBrowserItemNotification	message,
	DataBrowserItemDataRef		itemData )		// careful, itemData could be null!
{
	switch ( message )
		{
#if 0
		case kDataBrowserItemAdded:				// 1
		/* The specified item has been added to the browser */
		
		case kDataBrowserItemRemoved:			// 2
		/* The specified item has been removed from the browser */
		
		case kDataBrowserEditStarted:			// 3
		/* Starting an EditText session for specified item */
		
		case kDataBrowserEditStopped:			// 4
		/* Stopping an EditText session for specified item */
		
		case kDataBrowserItemSelected:			// 5
		/* Item has just been added to the selection set */
		
		case kDataBrowserItemDeselected:		// 6
		/* Item has just been removed from the selection set */
			break;
#endif
		
		case kDataBrowserItemDoubleClicked:		// 7
			DoDoubleClick( item, itemData );
			break;
		
		case kDataBrowserContainerOpened:		// 8
		/* Container is open */
			DoContainerOpened( item, itemData );
			break;
		
#if 0
		case kDataBrowserContainerClosing:		// 9
		/* Container is about to close (and will real soon now, y'all) */
		
		case kDataBrowserContainerClosed:		// 10
		/* Container is closed (y'all come back now!) */
		
		case kDataBrowserContainerSorting:		// 11
		/* Container is about to be sorted (lock any volatile properties) */
		
		case kDataBrowserContainerSorted:		// 12
		/* Container has been sorted (you may release any property locks) */
		
		case kDataBrowserUserStateChanged:		// 13
		/* The user has reformatted the view for the target */
#endif  // 0
		
		case kDataBrowserSelectionSetChanged:	// 14
		/* The selection set has been modified (net result may be the same) */
			DoSelectionSetChanged( item, itemData );
			break;
		
#if 0
		case kDataBrowserTargetChanged:			// 15
		/* The target has changed to the specified item */
		
		case kDataBrowserUserToggledContainer:	// 16
		/* _User_ requested container open/close state to be toggled */
#endif
		
		// there are no more cases (...yet)
		default:
			break;
		}
}


/*
**	BrowserList::ItemHelpContent()
**	provide help-tag content for a cell (or dispose of it).
**	This default implementation doesn't supply any content.
**	Subclasses may override this if they can do a better job of it.
*/
void
BrowserList::ItemHelpContent(
	DataBrowserItemID			/* item */,
	DataBrowserPropertyID		/* property */,
	HMContentRequest			inRequest,
	HMContentProvidedType *		outContentProvided,
	HMHelpContentRec *			/* ioHelpContent */ )
{
	// we have no clue
	if ( kHMSupplyContent == inRequest )
		*outContentProvided = kHMContentNotProvided;
}


/*
**	BrowserList::AddDBColumn()
**	subclass wishes to install a new column during setup.
**	inform the toolbox control.
*/
OSStatus
BrowserList::AddDBColumn(
	const DataBrowserListViewColumnDesc * inDesc,
	DataBrowserTableViewColumnIndex inPos /* = kDataBrowserListViewAppendColumn */ )
{
	OSStatus err = errDataBrowserNotConfigured;
	if ( view_ )
		{
			// I'm pretty sure it's a bug in the headers that the columnDesc* argument to
			// this routine isn't "const". So we'll live dangerously, and const_cast() it.
		err = AddDataBrowserListViewColumn( view_,
				const_cast<DataBrowserListViewColumnDesc *>( inDesc ),
				inPos );
		__Check_noErr( err );
		}
	return err;
}


/*
**	BrowserList::AddDBItems()
**	Subclasses call this when they wish to add items (i.e. rows) to the control.
**	For the kinds of flat lists we do, 'container' should always be 0 (the root).
*/
OSStatus
BrowserList::AddDBItems(
	DataBrowserItemID			container,
	UInt32						numItems,
	const DataBrowserItemID *	items,			 	// can be NULL
	DataBrowserPropertyID		preSortProperty /* = kDataBrowserItemNoProperty */ )
{
	OSStatus err = errDataBrowserNotConfigured;
	if ( view_ )
		{
		err = AddDataBrowserItems( view_, container, numItems, items, preSortProperty );
		__Check_noErr( err );
		}
	return err;
}


/*
**	BrowserList::RemoveDBItems()
**	Subclasses call this when they wish to remove items (i.e. rows) from the control.
**	For the kinds of flat lists we do, 'container' should always be 0 (the root).
*/
OSStatus
BrowserList::RemoveDBItems(
	DataBrowserItemID			container,
	UInt32						numItems,
	const DataBrowserItemID *	items,			 	// can be NULL
	DataBrowserPropertyID		preSortProperty /* = kDataBrowserItemNoProperty */ )
{
	OSStatus err = errDataBrowserNotConfigured;
	if ( view_ )
		{
		err = RemoveDataBrowserItems( view_, container, numItems, items, preSortProperty );
		__Check_noErr( err );
		}
	return err;
}


/*
**	BrowserList::UpdateDBItems()
**	Subclasses call this to inform the control that a range of items have changed
**	(in the backing store), and the that control should update the visible view.
**	'items' is either a pointer to an array of ItemIDs, or NULL
**	'numItems' is the size of that array, if there is one.
**	'preSortProp' is the ID of the column by which the supplied array items are
**	sorted, if they already are sorted.
**	'changedProp' is the column whose values have changed; if not provided, all
**	the columns need updating.
**	For the kinds of flat lists we do, 'container' should always be 0 (the root).
*/
OSStatus
BrowserList::UpdateDBItems(
	DataBrowserItemID			container,
	UInt32						numItems,
	const DataBrowserItemID *	items,				// can be NULL
	DataBrowserPropertyID		preSortProperty /* = kDataBrowserItemNoProperty */,
	DataBrowserPropertyID		changedProp /* = kDataBrowserItemNoProperty */ )
{
	OSStatus err = errDataBrowserNotConfigured;
	if ( view_ )
		{
		err = UpdateDataBrowserItems( view_, container, numItems, items,
					preSortProperty, changedProp );
		__Check_noErr( err );
		}
	return err;
}


/*
**	BrowserList::GetDBSelectedItem()
**	utility to locate the currently-selected item
*/
OSStatus
BrowserList::GetDBSelectedItem( DataBrowserItemID * outItem ) const
{
	// assume failure
	OSStatus err = errDataBrowserNotConfigured;
	DataBrowserItemID theItem = kDataBrowserNoItem;
	if ( view_ )
		{
#if 1
		// use Anchor API
		DataBrowserItemID frst, last;
		err = GetDataBrowserSelectionAnchor( view_, &frst, &last );
		__Check_noErr( err );
		if ( noErr == err )
			theItem = frst;
#else
		// use GetItems API
		Handle itemsH = NewHandle( 0 );
		if ( not itemsH )
			return memFullErr;
		
		err = GetDataBrowserItems( view_, kDataBrowserNoItem, false,
						kDataBrowserItemIsSelected,	itemsH );
		__Check_noErr( err );
		if ( noErr == err )
			{
			long nItems = GetHandleSize( itemsH ) / sizeof( *item );
			if ( 0 == nItems )
				theItem = kDataBrowserNoItem;
			else
			if ( 1 == nItems )
				theItem = * (const DataBrowserItemID *) *itemsH;
			else
				{
				// something is very wrong
# ifdef DEBUG_VERSION
				VDebugString( "unexpected selection size" );
# endif
				err = -1;
				}
			}
		DisposeHandle( itemsH );
#endif  // ! 1
		}
	if ( outItem )
		*outItem = theItem;
			
	return err;
}


/*
**	BrowserList::SetDBSelectedItem()
**	utility to change the selection
*/
OSStatus
BrowserList::SetDBSelectedItem( DataBrowserItemID item )
{
	OSStatus err = errDataBrowserNotConfigured;
	if ( view_ )
		{
		err = SetDataBrowserSelectedItems( view_, 1, &item, kDataBrowserItemsAssign );
		__Check_noErr( err );
		}
	
	return err;
}


/*
**	BrowserList::SetDBSelectedItems()
**	utility to select multiple items. (Yeah, I know...)
*/
OSStatus
BrowserList::SetDBSelectedItems(
	UInt32					nItems,
	DataBrowserItemID * 	items,
	DataBrowserSetOption	option /* = kDataBrowserItemsAssign */  )
{
	OSStatus err = errDataBrowserNotConfigured;
	if ( view_ )
		{
		err = SetDataBrowserSelectedItems( view_, nItems, items, option );
		__Check_noErr( err );
		}
	
	return err;
}


/*
**	BrowserList::GetDBPartBounds()
**	utility to provide geographical information about a database part
**	I forget why I needed this.
*/
OSStatus
BrowserList::GetDBPartBounds(
	DataBrowserItemID		  item,
	DataBrowserPropertyID     property,
	DataBrowserPropertyPart   part,
	Rect *                    bounds ) const
{
	OSStatus err = errDataBrowserNotConfigured;
	if ( view_ )
		{
		err = GetDataBrowserItemPartBounds( view_, item, property, part, bounds );
		__Check_noErr( err );
		}
	return err;
}


/*
**	BrowserList::SetDBHasScrollBars()
**	adjust scrollbar state-of-existence
*/
OSStatus
BrowserList::SetDBHasScrollBars( bool bHasHorz, bool bHasVert )
{
	OSStatus err = errDataBrowserNotConfigured;
	if ( view_ )
		{
		err = SetDataBrowserHasScrollBars( view_, bHasHorz, bHasVert );
		__Check_noErr( err );
		}
	return err;
}


/*
**	BrowserList::GetDBHasScrollBars()
**	determine current scrollbar state
*/
OSStatus
BrowserList::GetDBHasScrollBars( bool& bHasHorz, bool& bHasVert ) const
{
	OSStatus err = errDataBrowserNotConfigured;
	Boolean hasH = false, hasV = false;
	if ( view_ )
		{
		err = GetDataBrowserHasScrollBars( view_, &hasH, &hasV );
		__Check_noErr( err );
		}
	
	bHasHorz = hasH;
	bHasVert = hasV;
	return err;
}

	
/*
**	BrowserList::GetDBSortProperty()
**	utility to get current sorting key
*/
OSStatus
BrowserList::GetDBSortProperty( DataBrowserPropertyID * property ) const
{
	OSStatus err = errDataBrowserNotConfigured;
	if ( view_ )
		{
		DataBrowserPropertyID prop;
		err = GetDataBrowserSortProperty( view_, &prop );
		__Check_noErr( err );
		if ( noErr == err && property )
			*property = prop;
		}
	return err;
}


/*
**	BrowserList::SetDBSortProperty()
**	utility to change current sorting key
*/
OSStatus
BrowserList::SetDBSortProperty( DataBrowserPropertyID property )
{
	OSStatus err = errDataBrowserNotConfigured;
	if ( view_ )
		{
		err = SetDataBrowserSortProperty( view_, property );
		__Check_noErr( err );
		}
	return err;
}


/*
**	BrowserList::GetDBSortOrder()
**	utility to get current sorting direction
**	(for, presumably, the current sorting property?)
*/
OSStatus
BrowserList::GetDBSortOrder( DataBrowserSortOrder * order ) const
{
	OSStatus err = errDataBrowserNotConfigured;
	if ( view_ )
		{
		DataBrowserSortOrder ord;
		err = GetDataBrowserSortOrder( view_, &ord );
		__Check_noErr( err );
		if ( noErr == err && order )
			*order = ord;
		}
	return err;
}


/*
**	BrowserList::SetDBSortOrder()
**	utility to change current sorting direction
*/
OSStatus
BrowserList::SetDBSortOrder( DataBrowserSortOrder order )
{
	OSStatus err = errDataBrowserNotConfigured;
	if ( view_ )
		{
		err = SetDataBrowserSortOrder( view_, order );
		__Check_noErr( err );
		}
	return err;
}


/*
**	BrowserList::RevealDBSelection()
**	utility to scroll the current selection into view
*/
OSStatus
BrowserList::RevealDBSelection() const
{
	OSStatus err = errDataBrowserNotConfigured;
	if ( view_ )
		{
		DataBrowserItemID frst, last;
		err = GetDataBrowserSelectionAnchor( view_, &frst, &last );
		__Check_noErr( err );
		if ( noErr == err )
			{
			if ( frst && not last )
				last = frst;		// no biblical epigrams please
			
			err = RevealDataBrowserItem( view_,
						last,
						kDataBrowserItemNoProperty,		// whole row[?]
						kDataBrowserRevealWithoutSelecting );
			__Check_noErr( err );
			}
		}
	return err;
}


/*
**	BrowserList::SetScrollPosition()
**	utility to modify the scroll position
*/
OSStatus
BrowserList::SetScrollPosition( uint top, uint left ) const
{
	OSStatus err = errDataBrowserNotConfigured;
	if ( view_ )
		{
		err = SetDataBrowserScrollPosition( view_, top, left );
		__Check_noErr( err );
		}
	
	return err;
}


/*
**	BrowserList::OpenContainer()
**	utility to open a container.
**	Pointless for our flat lists, but hey, it was cheap.
*/
OSStatus
BrowserList::OpenContainer( DataBrowserItemID item /* =kDataBrowserNoItem */ )
{
	OSStatus err = errDataBrowserNotConfigured;
	if ( view_ )
		{
		err = OpenDataBrowserContainer( view_, item );
		__Check_noErr( err );
		}
	return err;
}



/*
**	SetOwnerTag()
**
**	place an identifying property tag on the Mac HIViewRef,
**	so we can find its BrowserList object instance later on
*/
OSStatus
SetOwnerTag( const BrowserList * bl, HIViewRef macCtl )
{
	OSStatus err = SetControlProperty( macCtl,
			kOwnedControlTagCreator,	// property creator
			kOwnedControlTagTag,		// property tag
			sizeof bl,
			&bl );
	__Check_noErr( err );
	return err;
}


/*
**	GetOwnerTag()
**
**	extract the identity tag from a Mac HIViewRef,
**	and determine the BrowserList object that corresponds to it.
*/
OSStatus
GetOwnerTag( HIViewRef macCtl, BrowserList ** bl )
{
	__Check( bl );
	if ( not bl )
		return paramErr;
	
	BrowserList * ptr = nullptr;
	OSStatus err = GetControlProperty( macCtl,
			kOwnedControlTagCreator, kOwnedControlTagTag,
			sizeof ptr, nullptr, &ptr );
	__Check_noErr( err );
	
	*bl = ptr;
	return err;
}


/*
**	GetAttributes()
**	OS 10.4+
*/
OSStatus
BrowserList::GetAttributes( OptionBits * outBits ) const
{
	__Check( outBits );
	if ( not outBits )
		return paramErr;
	
	if ( not view_ )
		return errDataBrowserNotConfigured;

	return DataBrowserGetAttributes( view_, outBits );
}


/*
**	SetAttributes()
**	OS 10.4+
*/
OSStatus
BrowserList::ChangeAttributes( OptionBits toSet, OptionBits toClear )
{
	if ( not view_ )
		return errDataBrowserNotConfigured;
	
	return DataBrowserChangeAttributes( view_, toSet, toClear );
}

#pragma mark -- CALLBACKS --


/*
**	BrowserList::InitCallbacks()	[static]
**
**	allocate the UPPs & install 'em
*/
OSStatus
BrowserList::InitCallbacks()
{
#if ! BROWSERLIST_USES_UPPS
	// the current platform doesn't require UPPs
	return noErr;
#else
	OSStatus err = noErr;
	
	// one-time creation of the UPPs
	if ( not itemDataCallback )
		{
		itemDataCallback		= NewDataBrowserItemDataUPP( DataCallback );
		itemCompareCallback		= NewDataBrowserItemCompareUPP( CompareCallback );
//		itemNotificationCallback = NewDataBrowserItemNotificationUPP( ItemNotifyCallback );
		
		// see the "Very Important Note about DataBrowserItemNotificationProcPtr"
		// in <ControlDefinitions.h> (or <HIDataBrowser.h>) for why we do this:
		itemNotificationWithItemUPP =
				NewDataBrowserItemNotificationWithItemUPP( ItemNotifyWithPointerCallback );
		
		itemHelpContentCallback = NewDataBrowserItemHelpContentUPP( ItemHelpContentCallback );
		
# if USE_EXTRA_CALLBACKS
		addDragItemCallback		= NewDataBrowserAddDragItemUPP( AddDragItemCallback );
		acceptDragCallback		= NewDataBrowserAcceptDragUPP( AcceptDragCallback );
		receiveDragCallback		= NewDataBrowserReceiveDragUPP( ReceiveDragCallback );
		postProcessDragCallback	= NewDataBrowserPostProcessDragUPP( PostProcessDragCallback );
		
		getContextualMenuCallback = NewDataBrowserGetContextualMenuUPP(
											GetContextualMenuCallback );
		selectContextualMenuCallback = NewDataBrowserSelectContextualMenuUPP(
											SelectContextualMenuCallback );
# endif  // USE_EXTRA_CALLBACKS
		
# if USE_CUSTOM_FORMAT
		drawItemCallback		= NewDataBrowserDrawItemUPP( DrawItemCallback );
		editTextCallback		= NewDataBrowserEditItemUPP( EditItemCallback );
		hitTestCallback			= NewDataBrowserHitTestUPP( HitTestCallback );
		trackingCallback		= NewDataBrowserTrackingUPP( TrackCallback );
		
		dragRegionCallback		= NewDataBrowserItemDragRgnUPP( ItemDragRegionCallback );
		acceptDragCallback		= NewDataBrowserItemAcceptDragUPP( ItemAcceptDragCallback );
		receiveDragCallback		= NewDataBrowserItemReceiveDragUPP( ItemReceiveDragCallback );
# endif  // USE_CUSTOM_FORMAT
		}
	
	return err;
#endif  // BROWSERLIST_USES_UPPS
}


/*
**	BrowserList::InstallCallbacks()
**
**	set up the various callback structures for the control
*/
OSStatus
BrowserList::InstallCallbacks()
{
	if ( not view_ )
		return paramErr;
	
	DataBrowserCallbacks cb;
	cb.version = kDataBrowserLatestCallbacks;
	
	OSStatus err = InitDataBrowserCallbacks( &cb );
	__Check_noErr( err );
	
	// 1-time UPP creation
	if ( noErr == err )
		err = InitCallbacks();
	
	// install the main callbacks
	if ( noErr == err )
		{
#if BROWSERLIST_USES_UPPS
		cb.u.v1.itemDataCallback = itemDataCallback;
		cb.u.v1.itemCompareCallback = itemCompareCallback;
		
		// again, consult the "Very Important Note about DataBrowserItemNotificationProcPtr"
		// in <ControlDefinitions.h>/<HIDataBrowser.h> for the rationale here:
		cb.u.v1.itemNotificationCallback =
				reinterpret_cast<DataBrowserItemNotificationUPP>( itemNotificationWithItemUPP );
		
		cb.u.v1.itemHelpContentCallback = itemHelpContentCallback;
#else
		cb.u.v1.itemDataCallback = DataCallback;
		cb.u.v1.itemCompareCallback = CompareCallback;
		
		// again, consult the "Very Important Note about DataBrowserItemNotificationProcPtr"
		// in <ControlDefinitions.h>/<HIDataBrowser.h> for the rationale here:
		cb.u.v1.itemNotificationCallback =
				reinterpret_cast<DataBrowserItemNotificationUPP>( ItemNotifyWithPointerCallback );
		
		cb.u.v1.itemHelpContentCallback = ItemHelpContentCallback;
#endif  // BROWSERLIST_USES_UPPS
		
#if USE_EXTRA_CALLBACKS
		// these are not often needed, so presently disabled to save time/space
# if BROWSERLIST_USES_UPPS
		cb.u.v1.addDragItemCallback = addDragItemCallback;
		cb.u.v1.acceptDragCallback = acceptDragCallback;
		cb.u.v1.receiveDragCallback = receiveDragCallback;
		cb.u.v1.postProcessDragCallback = postProcessDragCallback;
		
		cb.u.v1.getContextualMenuCallback = getContextualMenuCallback;
		cb.u.v1.selectContextualMenuCallback = selectContextualMenuCallback;
# else
		cb.u.v1.addDragItemCallback = AddDragItemCallback;
		cb.u.v1.acceptDragCallback = AcceptDragCallback;
		cb.u.v1.receiveDragCallback = ReceiveDragCallback;
		cb.u.v1.postProcessDragCallback = PostProcessDragCallback;
		
		cb.u.v1.getContextualMenuCallback = GetContextualMenuCallback;
		cb.u.v1.selectContextualMenuCallback = SelectContextualMenuCallback;
# endif  // BROWSERLIST_USES_UPPS
#endif	// USE_EXTRA_CALLBACKS
		
		err = SetDataBrowserCallbacks( view_, &cb );
		__Check_noErr( err );
		}
	
	// maybe do the custom-formatting ones as well
#if USE_CUSTOM_FORMAT
	DataBrowserCustomCallbacks cb2;
	cb2.version = kDataBrowserLatestCustomCallbacks;
	err = InitDataBrowserCustomCallbacks( &cb2 );
	__Check_noErr( err );
	if ( noErr == err )
		{
# if BROWSERLIST_USES_UPPS
		cb2.u.v1.drawItemCallback = drawItemCallback;
		cb2.u.v1.editTextCallback = editTextCallback;
		cb2.u.v1.hitTestCallback = hitTestCallback;
		cb2.u.v1.trackingCallback = trackingCallback;
		
		cb2.u.v1.dragRegionCallback = dragRegionCallback;
		cb2.u.v1.acceptDragCallback = acceptDragCallback;
		cb2.u.v1.receiveDragCallback = receiveDragCallback;
# else
		cb2.u.v1.drawItemCallback = DrawItemCallback;
		cb2.u.v1.editTextCallback = EditTextCallback;
		cb2.u.v1.hitTestCallback = HitTestCallback;
		cb2.u.v1.trackingCallback = TrackCallback;
		
		cb2.u.v1.dragRegionCallback = ItemDragRegionCallback;
		cb2.u.v1.acceptDragCallback = ItemAcceptDragCallback;
		cb2.u.v1.receiveDragCallback = ItemReceiveDragCallback;
# endif  // BROWSERLIST_USES_UPPS
		
		err = SetDataBrowserCustomCallbacks( view_, &cb2 );
		__Check_noErr( err );
		}
#endif	// USE_CUSTOM_FORMAT
	
	return err;
}


//
// Static callback dispatchers
//

/*
**	BrowserList::DataCallback()
**	dispatch a get/set-data request
*/
OSStatus
BrowserList::DataCallback(
	HIViewRef				browser,
	DataBrowserItemID		item,
	DataBrowserPropertyID	property,
	DataBrowserItemDataRef	itemData,
	Boolean					setValue )
{
	BrowserList * bl = nullptr;
	OSStatus err = GetOwnerTag( browser, &bl );
	if ( noErr == err && bl )
		{
		try
			{
			if ( setValue )
				err = bl->SetItemData( item, property, itemData );
			else
				err = bl->GetItemData( item, property, itemData );
			}
		catch (...) {}
		}
	return err;
}


/*
**	BrowserList::CompareCallback()
**	dispatch an item-comparison request
*/
Boolean
BrowserList::CompareCallback(
	HIViewRef				browser,
	DataBrowserItemID		itemOne,
	DataBrowserItemID		itemTwo,
	DataBrowserPropertyID	sortProperty )
{
	Boolean result = false;
	BrowserList * bl = nullptr;
	OSStatus err = GetOwnerTag( browser, &bl );
	if ( noErr == err && bl )
		{
		try
			{
			result = bl->ItemCompare( itemOne, itemTwo, sortProperty );
			}
		catch (...) {}
		}
	return result;
}


/*
**	BrowserList::ItemNotifyCallback()
**	dispatch an item notification
**	Only called (and installed!) on older systems that don't supply the 4th argument.
*/
void
BrowserList::ItemNotifyCallback(
	HIViewRef						browser,
	DataBrowserItemID				item,
	DataBrowserItemNotification		message )
{
	BrowserList * bl = nullptr;
	OSStatus err = GetOwnerTag( browser, &bl );
	if ( noErr == err && bl )
		{
		try
			{
			bl->ItemNotify( item, message, 0 );
			}
		catch (...) {}
		}
}


/*
**	BrowserList::ItemNotifyWithPointerCallback()
**	same as above, with extra info. Newer systems.
**	See the ridiculous note about this in <ControlDefinitions.h>.
*/
void
BrowserList::ItemNotifyWithPointerCallback(
	HIViewRef						browser,
	DataBrowserItemID				item,
	DataBrowserItemNotification		message,
	DataBrowserItemDataRef			itemData )
{
	BrowserList * bl = nullptr;
	OSStatus err = GetOwnerTag( browser, &bl );
	if ( noErr == err && bl )
		{
		try
			{
			bl->ItemNotify( item, message, itemData );
			}
		catch (...) {}
		}
}


/*
**	BrowserList::ItemHelpContentCallback()
**	dispatch a help-content request
*/
void
BrowserList::ItemHelpContentCallback(
	HIViewRef					browser,
	DataBrowserItemID			item,
	DataBrowserPropertyID		property,
	HMContentRequest			inRequest,
	HMContentProvidedType *		outContentProvided,
	HMHelpContentPtr			ioHelpContent )
{
	BrowserList * bl = nullptr;
	OSStatus err = GetOwnerTag( browser, &bl );
	if ( noErr == err && bl )
		{
		try
			{
			bl->ItemHelpContent( item, property, inRequest, outContentProvided, ioHelpContent );
			}
		catch (...) {}
		}
}


#if USE_EXTRA_CALLBACKS

/* Drag & Drop Processing */

/*
**	BrowserList::AddDragItemCallback()
**	dispatch an add-drag-items request
*/
Boolean
BrowserList::AddDragItemCallback(
	HIViewRef			theBrowser,
	DragReference		theDrag,
	DataBrowserItemID	item,
	ItemReference *		itemRef )
{
	Boolean result = false;
	BrowserList * bl = nullptr;
	OSStatus err = GetOwnerTag( browser, &bl );
	if ( noErr == err && bl )
		{
		try
			{
			result = bl->AddDragItem( theDrag, item, itemRef );
			}
		catch (...) {}
		}
	return result;
}


/*
**	BrowserList::AcceptDragCallback()
**	dispatch a drag-accept request
*/
Boolean
BrowserList::AcceptDragCallback(
	HIViewRef			theBrowser,
	DragReference		theDrag,
	DataBrowserItemID	item )
{
	Boolean result = false;
	BrowserList * bl = nullptr;
	OSStatus err = GetOwnerTag( browser, &bl );
	if ( noErr == err && bl )
		{
		try
			{
			result = bl->AcceptDrag( theDrag, item );
			}
		catch (...) {}
		}
	return result;
}


/*
**	BrowserList::ReceiveDragCallback()
**	dispatch a drag-receipt request
*/
Boolean
BrowserList::ReceiveDragCallback(
	HIViewRef			theBrowser,
	DragReference		theDrag,
	DataBrowserItemID	item )
{
	Boolean result = false;
	BrowserList * bl = nullptr;
	OSStatus err = GetOwnerTag( browser, &bl );
	if ( noErr == err && bl )
		{
		try
			{
			result = bl->ReceiveDrag( theDrag, item );
			}
		catch (...) {}
		}
	return result;
}


/*
**	BrowserList::PostProcessDragCallback()
**	dispatch a post-drag-cleanup request
*/
void
BrowserList::PostProcessDragCallback(
	HIViewRef		theBrowser,
	DragReference	theDrag,
	OSStatus		trackDragResult )
{
	BrowserList * bl = nullptr;
	OSStatus err = GetOwnerTag( browser, &bl );
	if ( noErr == err && bl )
		{
		try
			{
			bl->PostProcessDrag( theDrag, trackDragResult );
			}
		catch (...) {}
		}
}


/* Contextual Menu Support */

/*
**	BrowserList::GetContextualMenuCallback()
**	dispatch a "supply context menu" request
*/
void
BrowserList::GetContextualMenuCallback(
	HIViewRef		theBrowser,
	MenuRef *		menu,
	UInt32 *		helpType,
	CFStringRef *	helpItemString,
	AEDesc *		selection )
{
	BrowserList * bl = nullptr;
	OSStatus err = GetOwnerTag( browser, &bl );
	if ( noErr == err && bl )
		{
		try
			{
			bl->ItemData( menu, helpType, helpItemString, selection );
			}
		catch (...) {}
		}
}


/*
**	BrowserList::SelectContextualMenuCallback()
**	dispatch a "contextual menu item chosen" notification
*/
void
BrowserList::SelectContextualMenuCallback(
	HIViewRef		theBrowser,
	MenuRef			menu,
	UInt32			selectionType,
	SInt16			menuID,
	MenuItemIndex	menuItem )
{
	BrowserList * bl = nullptr;
	OSStatus err = GetOwnerTag( browser, &bl );
	if ( noErr == err && bl )
		{
		try
			{
			bl->SelectContextualMenu( menu, selectionType, menuID, menuItem );
			}
		catch (...) {}
		}
}
#endif	// USE_EXTRA_CALLBACKS


#if USE_CUSTOM_FORMAT

/* Custom format callbacks */

/*
**	BrowserList::DrawItemCallback()
**	dispatch a custom-draw request
*/
void
BrowserList::DrawItemCallback(
	HIViewRef				browser,
	DataBrowserItemID		item,
	DataBrowserPropertyID	property,
	DataBrowserItemState	itemState,
	const Rect *			theRect,
	SInt16					gdDepth,
	Boolean					colorDevice )
{
	BrowserList * bl = nullptr;
	OSStatus err = GetOwnerTag( browser, &bl );
	if ( noErr == err && bl )
		{
		try
			{
			bl->DrawItem( item, property, itemState, theRect, gdDepth, colorDevice );
			}
		catch (...) {}
		}
}


/*
**	BrowserList::EditItemCallback()
**	dispatch a custom-edit request
*/
Boolean
BrowserList::EditItemCallback(
	HIViewRef				browser,
	DataBrowserItemID		item,
	DataBrowserPropertyID	property,
	CFStringRef				theString,
	Rect *					maxEditTextRect,
	Boolean *				shrinkToFit )
{
	Boolean result = false;
	BrowserList * bl = nullptr;
	OSStatus err = GetOwnerTag( browser, &bl );
	if ( noErr == err && bl )
		{
		try
			{
			result = bl->EditItem( item, property, theString, maxEditTextRect, shrinkToFit );
			}
		catch (...) {}
		}
	return result;
}


/*
**	BrowserList::HitTestCallback()
**	dispatch a custom hit-test request
*/
Boolean
BrowserList::HitTestCallback(
	HIViewRef				browser,
	DataBrowserItemID		itemID,
	DataBrowserPropertyID	property,
	const Rect *			theRect,
	const Rect *			mouseRect )
{
	Boolean result = false;
	BrowserList * bl = nullptr;
	OSStatus err = GetOwnerTag( browser, &bl );
	if ( noErr == err && bl )
		{
		try
			{
			result = bl->HitTest( itemID, property, theRect, mouseRect );
			}
		catch (...) {}
		}
	return result;
}


/*
**	BrowserList::TrackCallback()
**	dispatch a custom-tracking request
*/
DataBrowserTrackingResult
BrowserList::TrackCallback(
	HIViewRef				browser,
	DataBrowserItemID		itemID,
	DataBrowserPropertyID	property,
	const Rect *			theRect,
	Point					startPt,
	EventModifiers			modifiers )
{
	DataBrowserTrackingResult result = kDataBrowserNothingHit;
	BrowserList * bl = nullptr;
	OSStatus err = GetOwnerTag( browser, &bl );
	if ( noErr == err && bl )
		{
		try
			{
			result = bl->Track( itemID, property, theRect, startPt, modifiers );
			}
		catch (...) {}
		}
	return result;
}


/*
**	BrowserList::ItemDragRgnCallback()
**	dispatch a custom drag-region request
*/
void
BrowserList::ItemDragRgnCallback(
	HIViewRef				browser,
	DataBrowserItemID		itemID,
	DataBrowserPropertyID	property,
	const Rect *			theRect,
	RgnHandle				dragRgn )
{
	BrowserList * bl = nullptr;
	OSStatus err = GetOwnerTag( browser, &bl );
	if ( noErr == err && bl )
		{
		try
			{
			bl->ItemDragRgn( itemID, property, theRect, dragRgn );
			}
		catch (...) {}
		}
}


/*
**	BrowserList::ItemAcceptDragCallback()
**	dispatch a custom drag-tasting request
*/
DataBrowserDragFlags
BrowserList::ItemAcceptDragCallback(
	HIViewRef				browser,
	DataBrowserItemID		itemID,
	DataBrowserPropertyID	property,
	const Rect *			theRect,
	DragReference			theDrag )
{
	DataBrowserDragFlags result = nullptr;
	BrowserList * bl = nullptr;
	OSStatus err = GetOwnerTag( browser, &bl );
	if ( noErr == err && bl )
		{
		try
			{
			result = bl->ItemAcceptDrag( itemID, property, theRect, theDrag );
			}
		catch (...) {}
		}
	return result;
}


/*
**	BrowserList::ItemReceiveDragCallback()
**	dispatch a custom drag-received notification
*/
Boolean
BrowserList::ItemReceiveDragCallback(
	HIViewRef				browser,
	DataBrowserItemID		itemID,
	DataBrowserPropertyID	property,
	DataBrowserDragFlags	dragFlags,
	DragReference			theDrag )
{
	Boolean result = false;
	BrowserList * bl = nullptr;
	OSStatus err = GetOwnerTag( browser, &bl );
	if ( noErr == err && bl )
		{
		try
			{
			result = bl->ItemReceiveDrag( itemID, property, dragFlags, theDrag );
			}
		catch (...) {}
		}
	return result;
}
#endif	// USE_CUSTOM_FORMAT

