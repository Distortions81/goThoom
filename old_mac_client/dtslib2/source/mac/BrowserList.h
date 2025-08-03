/*
**	BrowserList.h
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
**	kind of like a DTSDlgList, but uses a DataBrowser control.
**	Note that Dialog_mac*.cp knows absolutely nothing about these guys!
**
**	still needs some cleaning up and refactoring.
*/

#ifndef BROWSERLIST_H
#define BROWSERLIST_H


//
// set this to 1 if you need the extra standard callbacks (beyond
// Data, Notify, Compare, and Help): to wit, drag&drop and context menus.
// This is separate from the _custom format_ callbacks: for those, see next switch.
//
#define USE_EXTRA_CALLBACKS		0


//
//	set this to 1 if you require support for custom formatting
//
#define USE_CUSTOM_FORMAT		0


// FIXME:
// Maybe think about some kind of "data source"/delegate virtual base class
// to mediate between the BrowserList object and the real data store, whatever it may be. 


/*
**	Define the BrowserList class
*/
class BrowserList
{
	// the underlying HIDataBrowser object
	HIViewRef		view_;
	
public:
	// constructor/destructor
					BrowserList();
	virtual			~BrowserList();

	// no copying
private:
					BrowserList( const BrowserList& );
	BrowserList&	operator=( const BrowserList& );
	
public:
	// interface
//	OSStatus		Init( DTSView * inParent, HIViewRef inAdoptee = nullptr );
	OSStatus		InitFromNib( HIViewRef view );
	void			Exit();
	
		// these are here to enhance (hah) compatibility with older DTSDlgList-based,
		// code, but you're advised to start using Get/SetDBSelectedItem() instead.
	void			SetSelect( int item );
	int				GetSelect() const;
	
		// called when DB got a double click.
		// 'item' tells you what row was clicked.
		// Note: 'itemData' may be null, on older systems or in non-MachO builds.
	virtual void	DoDoubleClick( DataBrowserItemID /* item */,
								   DataBrowserItemDataRef /* itemData */ ) {}
	
		// called when a DB container was opened.
		// For flat ListViews and TableViews, this only occurs once,
		// for the root container (item == 0).
		// For ColumnViews and hierarchical ListViews, it can occur many many times.
		// On older systems & CFM, 'data' may be nil.
	virtual void	DoContainerOpened( DataBrowserItemID /* item */,
									   DataBrowserItemDataRef /* data */ ) {}
	
		// called when a DB's selection set has changed (although "net result may be the same")
	virtual void	DoSelectionSetChanged( DataBrowserItemID /* item */,
										   DataBrowserItemDataRef /* itemData */ ) {}
	
		// returns the currently selected item, if any
		// (right now we only support a single-selection model; the DB Control itself
		// has much more flexibility)
	OSStatus		GetDBSelectedItem( DataBrowserItemID * item ) const;
	
		// select 1 item
	OSStatus		SetDBSelectedItem( DataBrowserItemID item );
	
		// select multiple items
		// (Yeah, I know we promised we were imitating a single-selection DTSDlgList, but
		// this was cheap & easy to provide, just in case...)
	OSStatus		SetDBSelectedItems(
						UInt32 					nItems,
						DataBrowserItemID *		items,
						DataBrowserSetOption	option /* = kDataBrowserItemsAssign? */ );
	
		// set or change the current sort key and sort-order
	OSStatus		GetDBSortProperty( DataBrowserPropertyID * property ) const;
	OSStatus		SetDBSortProperty( DataBrowserPropertyID property );
	
	OSStatus		GetDBSortOrder( DataBrowserSortOrder 	 * order ) const;
	OSStatus		SetDBSortOrder( DataBrowserSortOrder	 order );
	
		// the currently-selected item will be scrolled into view
	OSStatus		RevealDBSelection() const;
	
		// modify the scrolling locattion
	OSStatus		SetScrollPosition( uint top, uint left ) const;
	
		// tell the control about newly added items, which it may need to display.
	OSStatus		AddDBItems(
						DataBrowserItemID			container,
						UInt32						numItems,
						const DataBrowserItemID *	items, // can be NULL
						DataBrowserPropertyID		preSortProperty = kDataBrowserItemNoProperty );
	
		// Similar, for a single item
	OSStatus		AddDBItem(
						DataBrowserItemID			container,
						DataBrowserItemID			item,
						DataBrowserPropertyID		preSortProperty = kDataBrowserItemNoProperty )
							{
							return AddDBItems( container, 1, &item, preSortProperty );
							}
	
		// call this to inform the control that some items have been removed from the 
		// backing store, and that it may need to update the screen.
	OSStatus		RemoveDBItems(
						DataBrowserItemID			container,
						UInt32						numItems,
						const DataBrowserItemID *	items, // can be NULL
						DataBrowserPropertyID		preSortProperty = kDataBrowserItemNoProperty );
	
		// convenience
	OSStatus		RemoveAllDBItems()
						{
						return RemoveDBItems( kDataBrowserNoItem, 0, nullptr );
						}
	
	
		// call this to inform the control that some existing items have changed,
		// and that it may need to update the screen.
	OSStatus		UpdateDBItems(
						DataBrowserItemID			container,
						UInt32						numItems,
						const DataBrowserItemID *	items, // can be NULL
						DataBrowserPropertyID		preSortProperty = kDataBrowserItemNoProperty,
						DataBrowserPropertyID		changedProp = kDataBrowserItemNoProperty );
	
	// private interface
protected:
		
#if 0	// not needed for browsers instantiated from nib files
		
		// Subclasses -must- provide this. Called after the DB is created, but before
		// it starts requesting data. This function must install definitions for the
		// desired columns (using AddDBColumn() to do the dirty work).
		// NB: BrowserLists instantiated from nib-files shouldn't provide this method,
		// since it would vitiate or even ruin all the design work done in the nib.
	virtual OSStatus	AddDBColumns() { return paramErr; }
#endif  // 0
	
		//
		// Utilities
		//
	
	// a new column will be added using the supplied description and location
	OSStatus		AddDBColumn(
						const DataBrowserListViewColumnDesc * inDesc,
						DataBrowserTableViewColumnIndex inIndex
							= kDataBrowserListViewAppendColumn );	// default: append
	
	// ask DB to "open" a container (default: root container)
	// (pretty much useless for flat lists)
	OSStatus		OpenContainer( DataBrowserItemID item = kDataBrowserNoItem );
	
	// DB attributes (only on OS 10.4+)
	OSStatus		ChangeAttributes( OptionBits toSet, OptionBits toClear );
	OSStatus		GetAttributes( OptionBits * outBits ) const;
	
		// allowable attributes for the above (these are not the true names!)
		// Consult the latest <ControlDefinitions.h> or <HIDataBrowser.h>
	enum
		{
		kDBAttrNone				= 0,
		kDBAttrColViewResize	= (1 << 0), // allow column-resizes to reshape enclosing _window_
		kDBAttrAlternateColors	= (1 << 1), // use fancy alternating-row-color display
		kDBAttrColDividers		= (1 << 2),	// draw vertical column separator borders
		kDBAttrAutoHideBars		= (1 << 3),	// hide scrollbars if not needed (10.5+)
		kDBAttrReserveGrowBox	= (1 << 4),	// don't intrude on growbox (10.5+)
		};
	
	
	// returns the bounding box for a specific part of a cell
	OSStatus		GetDBPartBounds( DataBrowserItemID		   item,
									 DataBrowserPropertyID     property,
									 DataBrowserPropertyPart   part,
									 Rect *                    bounds) const;
	
	OSStatus		SetDBHasScrollBars( bool bHasHorz, bool bHasVert );
	OSStatus		GetDBHasScrollBars( bool& bHasHorz, bool& bHasVert ) const;
	
		//
		// Callback interfaces
		//
	
	// called when the DB requires data for a cell.
	// Subclass MUST implement this.
	virtual OSStatus	GetItemData(
							DataBrowserItemID		item,
							DataBrowserPropertyID	property,
							DataBrowserItemDataRef	itemData ) = 0;
	
	// called after a user edit session has -changed- the data in a cell;
	// the implementation must adjust its backing store accordingly
	virtual OSStatus	SetItemData(
							DataBrowserItemID		/* item */,
							DataBrowserPropertyID	/* property */,
							DataBrowserItemDataRef	/* itemData */ )
								{
								return errDataBrowserPropertyNotSupported;
								};
	
	// called during a sort operation; return true if itemOne is
	// less than itemTwo in terms of the given sort property
	// Subclass SHOULD implement this, to get decent sorting.
	virtual bool		ItemCompare(
							DataBrowserItemID		itemOne,
							DataBrowserItemID		itemTwo,
							DataBrowserPropertyID	/* sortProperty */ )
								{
								// default implementation does the bare minimum.
								// Subclasses can do a better job.
								return itemOne < itemTwo;
								}
	
	// something interesting just happened.
		// The default implementation looks only for double-click events and selection-set
		// changes; it forwards those to DoDoubleClick() and DoSelectionSetChanged() respectively.
		// Don't rely on 'itemData', it may be null on older systems.
	virtual void		ItemNotify(
							DataBrowserItemID			item,
							DataBrowserItemNotification	message,
							DataBrowserItemDataRef		itemData );
	
	// Help Manager Support: provide (or dispose of) help-tag content for a given cell
	// Subclasses SHOULD implement this if they can provide any useful help at all.
	// Otherwise, the default behavior is to give no help.
	virtual void		ItemHelpContent(
							DataBrowserItemID		item,
							DataBrowserPropertyID	property,
							HMContentRequest		inRequest,
							HMContentProvidedType *	outContentProvided,
							HMHelpContentPtr		ioHelpContent );
	
#if USE_EXTRA_CALLBACKS
		// this part isn't finished yet...
		
/* Drag & Drop Processing */
	virtual bool		AddDragItem(
							DragReference		theDrag,
							DataBrowserItemID	item,
							ItemReference *		itemRef ) = 0;
	
	virtual bool		AcceptDrag(
							DragReference		/* theDrag */,
							DataBrowserItemID	/* item */ )
								{
								return false;
								}
	
	virtual Boolean		ReceiveDrag(
							DragReference		theDrag,
							DataBrowserItemID	item ) = 0;
	
	virtual void		PostProcessDrag(
							DragReference		theDrag,
							OSStatus			trackDragResult ) = 0;
	
/* Contextual Menu Support */
	virtual void		GetContextualMenu(
							MenuRef *			menu,
							UInt32 *			helpType,
							CFStringRef *		helpItemString,
							AEDesc *			selection ) = 0;
	
	virtual void		SelectContextualMenu(
							MenuRef				menu,
							UInt32				selectionType,
							int					menuID,
							int					menuItem ) = 0;
#endif	// USE_EXTRA_CALLBACKS


#if USE_CUSTOM_FORMAT

/* Custom format Support */
	virtual void		DrawItem(
							DataBrowserItemID		item,
							DataBrowserPropertyID	property,
							DataBrowserItemState	itemState,
							const Rect *			theRect,
							SInt16					gdDepth,
							bool					colorDevice );
	
	virtual bool		EditItem(
							DataBrowserItemID		item,
							DataBrowserPropertyID	property,
							CFStringRef				theString,
							Rect *					maxEditTextRect,
							Boolean *				shrinkToFit );
	
	virtual bool		HitTest(
							DataBrowserItemID		itemID,
							DataBrowserPropertyID	property,
							const Rect *			theRect,
							const Rect *			mouseRect );
	
	virtual DataBrowserTrackingResult Track(
							DataBrowserItemID		itemID,
							DataBrowserPropertyID	property,
							const Rect *			theRect,
							Point					startPt,
							EventModifiers			modifiers );
	
	virtual void		ItemDragRgn(
							DataBrowserItemID		itemID,
							DataBrowserPropertyID	property,
							const Rect *			theRect,
							RgnHandle				dragRgn );
	
	virtual DataBrowserDragFlags	ItemAcceptDrag(
							DataBrowserItemID		itemID,
							DataBrowserPropertyID	property,
							const Rect *			theRect,
							DragReference			theDrag );
	
	virtual bool		ItemReceiveDrag(
							DataBrowserItemID		itemID,
							DataBrowserPropertyID	property,
							DataBrowserDragFlags	dragFlags,
							DragReference			theDrag );
#endif	// USE_CUSTOM_FORMAT
private:	
	
	//
	// static callbacks. These are what get turned into UPPs;
	// all they actually do is dispatch back to the above virtual methods
	// (and act as firewalls for C++ exceptions).
	//
	static OSStatus			DataCallback(
								HIViewRef browser,
								DataBrowserItemID item,
								DataBrowserPropertyID property,
								DataBrowserItemDataRef itemData,
								Boolean setValue );
	
	static Boolean			CompareCallback(
								HIViewRef browser,
								DataBrowserItemID itemOne,
								DataBrowserItemID itemTwo,
								DataBrowserPropertyID sortProperty );
	
	static void				ItemNotifyCallback(
								HIViewRef browser,
								DataBrowserItemID item,
								DataBrowserItemNotification message );
	
	static void				ItemNotifyWithPointerCallback(
								HIViewRef browser,
								DataBrowserItemID item,
								DataBrowserItemNotification message,
								DataBrowserItemDataRef itemData );
	
	static void				ItemHelpContentCallback(
								HIViewRef				theBrowser,
								DataBrowserItemID		item,
								DataBrowserPropertyID	property,
								HMContentRequest		inRequest,
								HMContentProvidedType *	outContentProvided,
								HMHelpContentPtr		ioHelpContent );

#if USE_EXTRA_CALLBACKS
/* Drag & Drop Processing */
	static Boolean			AddDragItemCallback(
				HIViewRef			theBrowser,
				DragReference		theDrag,
				DataBrowserItemID	item,
				ItemReference *		itemRef );
	
	static Boolean			AcceptDragCallback(
				HIViewRef			theBrowser,
				DragReference		theDrag,
				DataBrowserItemID	item );
	
	static Boolean			ReceiveDragCallback(
				HIViewRef			theBrowser,
				DragReference		theDrag,
				DataBrowserItemID	item );
	
	static void				PostProcessDragCallback(
				HIViewRef			theBrowser,
				DragReference		theDrag,
				OSStatus			trackDragResult );

/* Contextual Menu Support */
	static void				GetContextualMenuCallback(
				HIViewRef			theBrowser,
				MenuRef *			menu,
				UInt32 *			helpType,
				CFStringRef *		helpItemString,
				AEDesc *			selection );
	
	static void				SelectContextualMenuCallback(
				HIViewRef			theBrowser,
				MenuRef				menu,
				UInt32				selectionType,
				SInt16				menuID,
				MenuItemIndex		menuItem );
#endif	// USE_EXTRA_CALLBACKS


#if USE_CUSTOM_FORMAT

/* Custom format callbacks */
	static void				DrawItemCallback(
				HIViewRef				browser,
				DataBrowserItemID		item,
				DataBrowserPropertyID	property,
				DataBrowserItemState	itemState,
				const Rect *			theRect,
				SInt16					gdDepth,
				Boolean					colorDevice );
	
	static Boolean			EditItemCallback(
				HIViewRef				browser,
				DataBrowserItemID		item,
				DataBrowserPropertyID	property,
				CFStringRef				theString,
				Rect *					maxEditTextRect,
				Boolean *				shrinkToFit );
	
	static Boolean			HitTestCallback(
				HIViewRef				browser,
				DataBrowserItemID		itemID,
				DataBrowserPropertyID	property,
				const Rect *			theRect,
				const Rect *			mouseRect );
	
	static DataBrowserTrackingResult TrackCallback(
				HIViewRef				browser,
				DataBrowserItemID		itemID,
				DataBrowserPropertyID	property,
				const Rect *			theRect,
				Point					startPt,
				EventModifiers			modifiers );
	
	static void				ItemDragRgnCallback(
				HIViewRef				browser,
				DataBrowserItemID		itemID,
				DataBrowserPropertyID	property,
				const Rect *			theRect,
				RgnHandle				dragRgn );
	
	static DataBrowserDragFlags	ItemAcceptDragCallback(
				HIViewRef				browser,
				DataBrowserItemID		itemID,
				DataBrowserPropertyID	property,
				const Rect *			theRect,
				DragReference			theDrag );
	
	static Boolean			ItemReceiveDragCallback(
				HIViewRef				browser,
				DataBrowserItemID		itemID,
				DataBrowserPropertyID	property,
				DataBrowserDragFlags	dragFlags,
				DragReference			theDrag );
#endif	// USE_CUSTOM_FORMAT
	

// UPPs aren't needed under Mach-O
#if __MACH__
# define BROWSERLIST_USES_UPPS		0
#else
# define BROWSERLIST_USES_UPPS		1
#endif
	
#if BROWSERLIST_USES_UPPS
	// UPPs for the static callbacks
	static DataBrowserItemDataUPP					itemDataCallback;
	static DataBrowserItemCompareUPP				itemCompareCallback;
//	static DataBrowserItemNotificationUPP			itemNotificationCallback;
	static DataBrowserItemNotificationWithItemUPP	itemNotificationWithItemUPP;
	static DataBrowserItemHelpContentUPP			itemHelpContentCallback;
	
#if USE_EXTRA_CALLBACKS
	static DataBrowserAddDragItemUPP				addDragItemCallback;
	static DataBrowserAcceptDragUPP					acceptDragCallback;
	static DataBrowserReceiveDragUPP				receiveDragCallback;
	static DataBrowserPostProcessDragUPP			postProcessDragCallback;
	
	static DataBrowserGetContextualMenuUPP			getContextualMenuCallback;
	static DataBrowserSelectContextualMenuUPP		selectContextualMenuCallback;
#endif	// USE_EXTRA_CALLBACKS
	
#if USE_CUSTOM_FORMAT
	static DataBrowserDrawItemUPP					drawItemCallback;
	static DataBrowserEditItemUPP					editTextCallback;
	static DataBrowserHitTestUPP					hitTestCallback;
	static DataBrowserTrackingUPP					trackingCallback;
	
	static DataBrowserItemDragRgnUPP				dragRegionCallback;
	static DataBrowserItemAcceptDragUPP				acceptDragCallback;
	static DataBrowserItemReceiveDragUPP			receiveDragCallback;
#endif	// USE_CUSTOM_FORMAT
#endif  // BROWSERLIST_USES_UPPS
	
	// callback installer
	// allocates and initializes the callback UPPs and determines
	// which of the two ItemNotifiers is appropriate for the current environment.
	static OSStatus		InitCallbacks();
	OSStatus			InstallCallbacks();
	
protected:
	operator HIViewRef() const { return view_; }
};

#endif	// BROWSERLIST_H

