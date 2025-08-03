/*
**	Dialog_mac.cp		dtslib2
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

#include "View_mac.h"
#include "Dialog_mac.h"
#include "Shell_mac.h"


/*
**	Entry Routines
**
**	DTSDlgView
**	GetItem()
**
**	GenericError();
**	GenericOkCancel();
**	GenericYesNoCancel();
*/


/*
**	Definitions
*/
// item-type mask for distinguishing between kControlDialogItem items (viz., buttons, checkboxes,
// radios, and bona-fide CNTL controls) and the other sorts of things found in dialogs (viz.,
// static- or edit-text fields, icons, user items, etc.)
const ushort kItemTypeControlMask	= ~( kControlDialogItem - 1 );

# if MAC_OS_X_VERSION_MAX_ALLOWED <= 1040
enum { kEventMouseScroll = 11 };   // not in 10.4 headers
# endif


/*
**	struct DlgKey holds key-equivalents for dialog items
*/
#pragma pack(push, 2)
struct DlgKey
{
	uchar	dkKey;
	uchar	dkMask;
	uchar	dkModifiers;
	uchar	dkItem;
};
#pragma pack(pop)


/*
**	class DTSDlgViewPriv
**	implementation base class for dialogs
*/
class DTSDlgViewPriv : public CarbonEventResponder 
{
public:
	DialogRef	dlgDialog;
	DlgKey *	dlgKeys;
	DTSPoint	dlgWhere;
	UInt32		dlgWhen;
	int			dlgDefBtn;
	int			dlgCancelBtn;
	int			dlgItemHit;
	int			dlgModifiers;
	bool		dlgDismiss;
	
public:

	// constructor/destructor
							DTSDlgViewPriv();
	virtual					~DTSDlgViewPriv();
	
private:
	// declared but not defined
							DTSDlgViewPriv( const DTSDlgViewPriv & rhs );
	DTSDlgViewPriv& 		operator=( const DTSDlgViewPriv & rhs );
	
public:
	static void				InitDlgItems( DialogRef dlg );
	static Boolean			ModalFilter( DialogRef, EventRecord *, DialogItemIndex * );
	
	static HIViewRef		GetItem( const DTSDlgView * dtsdlg, int item, bool embedded = false );
	
private:
	virtual OSStatus		HandleEvent( CarbonEvent& );
	
		// user-item proc for user items
	static void				DrawUserItem( DialogRef, DialogItemIndex );
};


// used only in editor in a couple of places
/*
**	DTSDlgListPriv
**
**	a list-box element inside a dialog
*/
class DTSDlgListPriv
{
	DTSDlgListPriv *	dlgNext;
	int					dlgHideLevel;		// don't draw when > 0
	
public:
	const DTSDlgView *	dlgView;
	ListHandle			dlgListHdl;
	int					dlgItem;
	bool				dlgDoubleClick;
	bool				dlgIsListControl;	// Appearance list control, or vanilla list mgr?
	
	// constructor/destructor
							DTSDlgListPriv();
							~DTSDlgListPriv();
private:
	// declared but not defined
							DTSDlgListPriv( const DTSDlgListPriv& rhs );
	DTSDlgListPriv&			operator=( const DTSDlgListPriv& rhs );
	
public:
	// interface
	void					Exit();
	void					Click();
	int						GetSelect() const;
	void					HideDrawing();
	void					ShowDrawing();
	bool					IsDrawingEnabled() const { return dlgHideLevel == 0; }
	
	static DTSDlgListPriv *	FindList( DialogRef, int item );
	static void				DrawListItem( DialogRef dlg, DialogItemIndex item );
	static UserItemUPP		gDrawListItemUPP;
	
private:
	static DTSDlgListPriv *	gRootList;				// linked list of DlgListPrivs
};


// generic dialog kinds
enum GDKind
{
	kGDError,
	kGDOkCancel,
	kGDYesNoCancel
};


/*
**	Internal Routines
*/
static int				VGenericDialog( GDKind kind, const char * format, va_list params );


/*
**	Internal Variables
*/
static DTSDlgView *	gRootDlgView;					// stack of DTS dialogs


/*
**	Class variables
*/
	// linked list of DlgListPrivs
DTSDlgListPriv *		DTSDlgListPriv::gRootList;
	// user-item proc for List items in dialogs
UserItemUPP				DTSDlgListPriv::gDrawListItemUPP;


const EventTypeSpec kScrollingEvents[] =
{
	{ kEventClassMouse, kEventMouseScroll },
	{ kEventClassMouse, kEventMouseWheelMoved }
};


/*
**	DTSDlgViewPriv
*/
DTSDefineImplementFirmNoCopy(DTSDlgViewPriv)


/*
**	DTSDlgViewPriv::DTSDlgViewPriv()
*/
DTSDlgViewPriv::DTSDlgViewPriv() :
	dlgDialog( nullptr ),
	dlgKeys( nullptr ),
	dlgDefBtn( 0 ),
	dlgCancelBtn( 1 ),
	dlgItemHit( 0 ),
	dlgDismiss( false )
{
}


/*
**	DTSDlgViewPriv::~DTSDlgViewPriv()
*/
DTSDlgViewPriv::~DTSDlgViewPriv()
{
	delete[] reinterpret_cast<char *>( dlgKeys );
}


/*
**	DTSDlgView::Present
*/
void
DTSDlgView::Present( int id )
{
	DTSViewPriv * vp = DTSView::priv.p;
	if ( not vp )
		return;
	
	DTSDlgViewPriv * dp = DTSDlgView::priv.p;
	if ( not dp )
		return;
	
#ifndef AVOID_QUICKDRAW
	GrafPtr	oldPort;
	GetPort( &oldPort );
#endif
	
	// create the dialog
	DialogRef dlg = GetNewDialog( id, nullptr, kFirstWindowOfClass );
	CheckPointer( dlg );
	
	// associate DTS object with Mac object, via refcon, and vice-versa
	WindowRef dlgwin = ::GetDialogWindow( dlg );
	::SetWRefCon( dlgwin, reinterpret_cast<long>( this ) );		// SRefCon
	dp->dlgDialog = dlg;
	
	// load the optional keyboard shortcuts
		{
		Handle dkH = ::GetResource( 'DlgK', id );
		if ( dkH && *dkH )
			{
			::Size size = ::GetHandleSize( dkH );
			if ( size > 0 )
				{
				DlgKey * dlgk = reinterpret_cast<DlgKey *>( NEW_TAG("Dialog Keys") char[ size ] );
				if ( dlgk )
					{
					memcpy( dlgk, *dkH, size );
					dp->dlgKeys = dlgk;
					}
				}
			}
		}
	
	// link into dialog chain
	DTSDlgView * saveView = gRootDlgView;
	gRootDlgView = this;
	
	// initialize the DTSView
	CGrafPtr dlgPort = GetDialogPort( dlg );
	vp->InitViewPort( (GrafPtr) dlgPort, nullptr );
	
	// be extra sure the port is correct
	SetPortDialogPort( dlg );
	
	// install special items (the DTSView must be correct before calling this)
	DTSDlgViewPriv::InitDlgItems( dlg );
	
	// subclass override
	Init();
	
	DoActivate( ::FrontNonFloatingWindow(), false );
	
	ShowWindow( dlgwin );
	
	GetWindowPortBounds( dlgwin, DTS2Mac(&vp->viewBounds) );
#ifndef AVOID_QUICKDRAW
	vp->viewContent.Set( &vp->viewBounds );
#endif
	
	// ensure we have the modal-filter and list-redraw UPPs
#if TARGET_RT_MAC_MACHO
	const ModalFilterUPP modalUPP = DTSDlgViewPriv::ModalFilter;
#else
	static ModalFilterUPP modalUPP;
	if ( not modalUPP )
		{
		modalUPP = NewModalFilterUPP( DTSDlgViewPriv::ModalFilter );
		__Check( modalUPP );
		}
#endif  // ! MACHO
	
	if ( not DTSDlgListPriv::gDrawListItemUPP )
		{
		DTSDlgListPriv::gDrawListItemUPP = NewUserItemUPP( DTSDlgListPriv::DrawListItem );
		__Check( DTSDlgListPriv::gDrawListItemUPP );
		}
	
	dp->dlgDismiss = false;
	while ( not dp->dlgDismiss )
		{
		DialogItemIndex itemHit = -1;
		dp->dlgItemHit = -1;
		ModalDialog( modalUPP, &itemHit );
		dp->dlgItemHit = itemHit;
		
		// handle hits on items
		DialogItemType kind;
		Handle hdl;
		Rect box;
		GetDialogItem( dlg, itemHit, &kind, &hdl, &box );
		
		// send clicks to DTSDlgLists
		if ( Handle(DTSDlgListPriv::gDrawListItemUPP) == hdl
		||	 kResourceControlDialogItem == kind )
			{
			if ( DTSDlgListPriv * list = DTSDlgListPriv::FindList( dlg, itemHit ) )
				list->Click();
			}
		
		StDrawEnv saveEnv( this );
		
		// inform the item it has been hit
		Hit( itemHit );
		}
	
	Exit();
	dp->RemoveEventTypes( GetEventTypeCount( kScrollingEvents ), kScrollingEvents );
	dp->RemoveHandler();
	DisposeDialog( dlg );
	dp->dlgDialog = nullptr;		// just in case
	
	gRootDlgView = saveView;
#ifndef AVOID_QUICKDRAW
	SetPort( oldPort );
#endif
	
	DTSShowArrowCursor();
}


/*
**	InitDlgItems()	[static]
*/
void
DTSDlgViewPriv::InitDlgItems( DialogRef dlg )
{
	DTSDlgView * dtsdlg = reinterpret_cast<DTSDlgView *>( ::GetWRefCon( ::GetDialogWindow( dlg )));
	if ( not dtsdlg )
		return;
	
	DTSDlgViewPriv * dp = dtsdlg->priv.p;
	if ( not dp )
		return;
	
	// carbon events
	OSStatus err = dp->InitTarget( GetWindowEventTarget( GetDialogWindow( dlg ) ) );
	if ( noErr == err )
		err = dp->AddEventTypes( GetEventTypeCount( kScrollingEvents ), kScrollingEvents );
	
	bool bTracksCursor = false;
	
	// init user-items; check for edit-text fields; set default cancel button
	int numItems = CountDITL( dlg );
	for ( int nnn = kStdCancelItemIndex;  nnn <= numItems;  ++nnn )
		{
		DialogItemType kind;
		Handle hdl;
		Rect box;
		::GetDialogItem( dlg, nnn, &kind, &hdl, &box );
		
		// look for user items
		if ( kUserDialogItem == (kind & ~kItemDisableBit) )
			{
			// install the generic user item procedure
#if TARGET_RT_MAC_MACHO
			const UserItemUPP upp = DTSDlgViewPriv::DrawUserItem;
#else
			static UserItemUPP upp;
			if ( not upp )
				{
				upp = NewUserItemUPP( DrawUserItem );
				__Check( upp );
				}
#endif
			SetDialogItem( dlg, nnn, kind, reinterpret_cast<Handle>( upp ), &box );
			}
		
		// hilite inactive controls appropriately
		if ( (kind & kItemTypeControlMask) == (kControlDialogItem | kItemDisableBit) )
			{
			__Verify_noErr( DeactivateControl( reinterpret_cast<HIViewRef>( hdl ) ) );
//			HiliteControl( reinterpret_cast<HIViewRef>( hdl ), kControlInactivePart );
			}
		
		// BOGUS: check for a presumed cancel button
		if ( kStdCancelItemIndex == nnn		// ie 2
		&&   kButtonDialogItem == kind )
			{
			dtsdlg->SetCancelItem( nnn );
			}
		
		// check for edit-text items (FIXME: fails to spot edit-via-CNTL items)
		if ( kEditTextDialogItem == (kind & ~kItemDisableBit) )
			bTracksCursor = true;
		}
	
	if ( bTracksCursor )
		__Verify_noErr( SetDialogTracksCursor( dlg, true ) );
}


/*
**	DTSDlgViewPriv::GetItem()	[static]
*/
HIViewRef
DTSDlgViewPriv::GetItem( const DTSDlgView * dtsdlg, int item, bool embeddedOnly /* = false */ )
{
	if ( not dtsdlg )
		return nullptr;
	
	DialogRef macdlg = DTS2Mac( dtsdlg );
	if ( not macdlg )
		return nullptr;
	
	// try to find a root control
	HIViewRef ctl = nullptr;
	{
	HIViewRef root;
	OSStatus err = GetRootControl( GetDialogWindow( macdlg ), &root );
	if ( noErr == err )
		{
		// found a root: so there must be an embedding.
		// just get the desired control and we're done
		err = GetDialogItemAsControl( macdlg, item, &ctl );
		
		if ( noErr == err && ctl )
			return ctl;
		
		if ( embeddedOnly )
			return (noErr == err) ? ctl : nullptr;
		}
	// fall thru to non-AM case
	}
	
	// non-Appearance: the dialog item's handle is (usually) a HIViewRef
	// or, we do have AM but the 'dlgx' resource doesn't call for a hierarchy
	DialogItemType kind;
	Handle handle;
	Rect box;
	GetDialogItem( macdlg, item, &kind, &handle, &box );
	ctl = reinterpret_cast<HIViewRef>( handle );
	return ctl;
}


/*
**	DrawUserItem()		[static]
*/
void
DTSDlgViewPriv::DrawUserItem( DialogRef dlg, DialogItemIndex item )
{
	DTSDlgView * dtsdlg = reinterpret_cast<DTSDlgView *>(
							GetWRefCon( GetDialogWindow( dlg ) ) );
	StDrawEnv saveEnv( dtsdlg );
	
	DialogItemType kind;
	Handle hdl;
	DTSRect box;
	GetDialogItem( dlg, item, &kind, &hdl, DTS2Mac(&box) );
	dtsdlg->DrawItem( item, &box );
}


/*
**	DTSModalFilter()	[static]
*/
Boolean
DTSDlgViewPriv::ModalFilter( DialogRef dlg, EventRecord * event, DialogItemIndex * itemHit )
{
	DTSDlgView * dlgView = reinterpret_cast<DTSDlgView *>(
							::GetWRefCon( ::GetDialogWindow( dlg ) ) );
	if ( not dlgView )
		return false;
	
	DTSDlgViewPriv * dp = dlgView->priv.p;
	if ( not dp )
		return false;
	
	bool result = false;
	
	StDrawEnv saveEnv( dlgView );
	
#ifndef AVOID_QUICKDRAW
	Point localMouse = event->where;
	QDGlobalToLocalPoint( GetDialogPort( dlg ), &localMouse );
#else
	HIPoint hwhere = CGPointMake( event->where.h, event->where.v );
	HIViewRef contentVu = nullptr;
	__Verify_noErr( HIViewFindByID( HIViewGetRoot( GetDialogWindow( dlg ) ),
								   kHIViewWindowContentID, &contentVu ) );
	if ( contentVu )
		HIPointConvert( &hwhere, kHICoordSpace72DPIGlobal, nullptr, kHICoordSpaceView, contentVu );
	Point localMouse;
	localMouse.h = hwhere.x;
	localMouse.v = hwhere.y;
#endif  // AVOID_QUICKDRAW
	
	dp->dlgWhere.Set( localMouse.h, localMouse.v );
	dp->dlgWhen      = event->when;
	dp->dlgModifiers = Mac2DTSModifiers( event );
	WindowRef evtWin = reinterpret_cast<WindowRef>( event->message );
	
	switch ( event->what )
		{
		case nullEvent:
			dlgView->Idle();
			if ( dp->dlgDismiss )
				result = true;
			break;
		
		case mouseDown:
			break;
		
		case keyDown:
		case autoKey:
			result = dlgView->KeyFilter( ::GetDialogKeyboardFocusItem(dlg),
								(uchar) event->message, dp->dlgModifiers );
			if ( result )
				event->what = nullEvent;
			else
				result = dlgView->KeyStroke( (uchar) event->message, dp->dlgModifiers );
			break;
		
		case updateEvt:
			if ( GetDialogWindow( dlg ) != evtWin )
				{
				gEvent = *event;
				::HandleEvent();
				}
			break;
		
#if 1
// I think this is the proper behavior, but we shall see...
		case activateEvt:
			if ( dialogKind == ::GetWindowKind( evtWin ) )
				{
				SetPortWindowPort( evtWin );
				
				Rect b;
				GetWindowPortBounds( evtWin, &b );
				InvalWindowRect( evtWin, &b );
				
 				SetPortDialogPort( dlg );
				}
			break;
#endif	// 1
		
		default:
			gEvent = *event;
			::HandleEvent();
			break;
		}
	
	if ( not result )
		{
		DialogItemIndex stdItemHit = -1;
		result = StdFilterProc( dlg, event, &stdItemHit );
		dp->dlgItemHit = stdItemHit;
		}
	
	*itemHit = dp->dlgItemHit;
	
	return result;
}


/*
**	DTSDlgView::KeyStroke()
*/
bool
DTSDlgView::KeyStroke( int ch, uint modifiers )
{
	DTSDlgViewPriv * dp = priv.p;
	if ( not dp )
		return false;
	
	const DlgKey * keys = dp->dlgKeys;
	if ( not keys )
		return false;
	
	bool result = false;
	for(;;)
		{
		int test = keys->dkKey;
		if ( not test )
			break;
		
		if ( test == ch
		&&   keys->dkModifiers == (modifiers & keys->dkMask) )
			{
			int itemHit = keys->dkItem;
			
			// watch out for disabled items
			Rect r;
			Handle h;
			DialogItemType t;
			GetDialogItem( DTS2Mac(this), itemHit, &t, &h, &r );
			if ( not (t & kItemDisableBit) )
				{
				dp->dlgItemHit = itemHit;
				result = true;
				break;
				}
			}
		++keys;
		}
	
	return result;
}


/*
**	DTSDlgView::Dismiss()
*/
void
DTSDlgView::Dismiss( int item )
{
	if ( DTSDlgViewPriv * dp = priv.p )
		{
		dp->dlgDismiss = true;
		dp->dlgItemHit = item;
		}
}


/*
**	DTSDlgView::SetValue()
*/
void
DTSDlgView::SetValue( int item, int value ) const
{
	const DTSDlgViewPriv * dp = priv.p;
	if ( not dp )
		return;
	
	DialogItemType kind = -1;
	Handle hdl;
	Rect box;
	GetDialogItem( DTS2Mac(this), item, &kind, &hdl, &box );
	
	// can't set a value if the item is not a
	// DITL pseudo-control (button, checkbox, radio)
	// or a really-truly control.
	if ( kControlDialogItem == (kind & kItemTypeControlMask) )
		{
		if ( HIViewRef ctl = DTSDlgViewPriv::GetItem( this, item ) )
			__Verify_noErr( HIViewSetValue( ctl, value ) );
		}
}


/*
**	DTSDlgView::GetValue()
*/
int
DTSDlgView::GetValue( int item ) const
{
	const DTSDlgViewPriv * dp = priv.p;
	if ( not dp )
		return 0;
	
	DialogItemType kind = -1;
	Handle hdl;
	Rect box;
	::GetDialogItem( DTS2Mac(this), item, &kind, &hdl, &box );
	
	// only DITL pseudo-controls, and genuine ones, have control values
	int value = 0;
	if ( kControlDialogItem == (kind & kItemTypeControlMask) )
		{
		if ( HIViewRef ctl = DTSDlgViewPriv::GetItem( this, item ) )
			value = HIViewGetValue( ctl );
		}
	return value;
}


/*
**	DTSDlgView::SetText()
*/
void
DTSDlgView::SetText( int item, const char * text ) const
{
	if ( CFStringRef cs = CreateCFString( text ) )
		{
		if ( HIViewRef ctl = DTSDlgViewPriv::GetItem( this, item ) )
			{
			if ( noErr == HIViewSetText( ctl, cs ) )
				{
//				__Verify_noErr( HIViewRender( ctl ) );
				// force a redraw
				Handle hdl;
				Rect box;
				DialogItemType kind;
				GetDialogItem( DTS2Mac(this), item, &kind, &hdl, &box );
				InvalWindowRect( GetDialogWindow(DTS2Mac(this)), &box );
				}
			}
		CFRelease( cs );
		}
}


/*
**	DTSDlgView::GetText()
*/
void
DTSDlgView::GetText( int item, size_t maxLength, char * text ) const
{
	*text = '\0';
	
	const DTSDlgViewPriv * dp = priv.p;
	if ( not dp )
		return;
	
	if ( HIViewRef ctl = DTSDlgViewPriv::GetItem( this, item ) )
		{
		if ( CFStringRef cs = HIViewCopyText( ctl ) )
			{
			CFStringGetCString( cs, text, maxLength, kCFStringEncodingMacRoman );
			CFRelease( cs );
			}
		}
}


/*
**	DTSDlgView::SetNumber()
*/
void
DTSDlgView::SetNumber( int item, int number ) const
{
	char buff[ 32 ];
	snprintf( buff, sizeof buff, "%d", number );
	SetText( item, buff );
}


/*
**	DTSDlgView::GetNumber()
*/
int
DTSDlgView::GetNumber( int item ) const
{
	char buff[ 32 ];
	GetText( item, sizeof buff, buff );
	
	// this causes the function to return 0x80000000 if sscanf() failed to parse anything
	int number;
	if ( 1 != sscanf( buff, "%d", &number ) )
		number = static_cast<int>( 0x80000000 );
	
	return number;
}


/*
**	DTSDlgView::SetFloat()
*/
void
DTSDlgView::SetFloat( int item, double number ) const
{
	// print the floating point number with lots of extra decimal places
	char buff[ 32 ];
	int len = snprintf( buff, sizeof buff, "%.8lf", number );
	
	// remove all trailing zeros
	char * ptr = &buff[ len - 1 ];
	while ( '0' == *ptr )			// note character '0' == 0x30, not 0x0 !
		{
		*ptr = '\0';
		--ptr;
		}
	
	// set the text to the number
	SetText( item, buff );
}


/*
**	DTSDlgView::GetFloat()
*/
double
DTSDlgView::GetFloat( int item ) const
{
	char buff[ 32 ];
	GetText( item, sizeof buff, buff );
	
	double number = 0;
	sscanf( buff, "%lf", &number );
	
	return number;
}


/*
**	DTSDlgView::SelectText()
*/
void
DTSDlgView::SelectText( int item, int start, int stop ) const
{
	DialogItemType kind;
	Handle hdl;
	Rect box;
	DialogRef dlg = DTS2Mac(this);
	
	::GetDialogItem( dlg, item, &kind, &hdl, &box );
	kind &= ~kItemDisableBit;
	
	if ( kEditTextDialogItem == kind )
		::SelectDialogItemText( dlg, item, start, stop );
}


/*
**	DTSDlgView::GetSelect()
*/
void
DTSDlgView::GetSelect( int * start, int * stop ) const
{
	if ( TEHandle hte = ::GetDialogTextEditHandle( DTS2Mac(this) ) )
		{
		TEPtr pte = *hte;
		if ( start )
			*start = pte->selStart;
		if ( stop )
			*stop  = pte->selEnd;
		}
	else
		{
		if ( start )
			*start = 0;
		if ( stop )
			*stop  = 0;
		}
}


/*
**	DTSDlgView::GetBoundRect()
*/
void
DTSDlgView::GetBoundRect( int item, DTSRect * bounds ) const
{
	if ( not bounds )
		return;
	
	DialogItemType kind;
	Handle hdl;
	::GetDialogItem( DTS2Mac(this), item, &kind, &hdl, DTS2Mac(bounds) );
}


/*
**	DTSDlgView::RedrawItem()
*/
void
DTSDlgView::RedrawItem( int item )
{
	StDrawEnv saveEnv( this );
	
	DTSRect	r;
	GetBoundRect( item, &r );
	DrawItem( item, &r );
}


/*
**	DTSDlgView::CheckRadio()
*/
void
DTSDlgView::CheckRadio( int checked, int min, int max ) const
{
	for ( ;  min <= max;  ++min )
		{
		int value = 0;
		if ( min == checked )
			value = 1;
		SetValue( min, value );
		}
}


/*
**	DTSDlgView::Enable()
*/
void
DTSDlgView::Enable( int item ) const
{
	if ( HIViewRef ctl = DTSDlgViewPriv::GetItem( this, item ) )
		{
		__Verify_noErr( EnableControl( ctl ) );
		}
}


/*
**	DTSDlgView::Disable()
*/
void
DTSDlgView::Disable( int item ) const
{
	if ( HIViewRef ctl = DTSDlgViewPriv::GetItem( this, item ) )
		{
		__Verify_noErr( DisableControl( ctl ) );
		}
}


/*
**	DTSDlgView::ShowItem()
*/
void
DTSDlgView::ShowItem( int item ) const
{
	ShowDialogItem( DTS2Mac(this), item );
}


/*
**	DTSDlgView::HideItem()
*/
void
DTSDlgView::HideItem( int item ) const
{
	HideDialogItem( DTS2Mac(this), item );
}


/*
**	DTSDlgView::SetDefaultItem()
*/
void
DTSDlgView::SetDefaultItem( int item ) const
{
	DTSDlgViewPriv * dp = priv.p;
	if ( not dp )
		return;
	
	dp->dlgDefBtn = item;
	if ( item > 0 )
		__Verify_noErr( SetDialogDefaultItem( DTS2Mac(this), item ) );
}


/*
**	DTSDlgView::SetCancelItem()
*/
void
DTSDlgView::SetCancelItem( int item ) const
{
	DTSDlgViewPriv * dp = priv.p;
	if ( not dp )
		return;

	dp->dlgCancelBtn = item;
	if ( item > 0 )
		__Verify_noErr( SetDialogCancelItem( DTS2Mac(this), item ) );
}


/*
**	DTSDlgView::SetPopupItem()
**
**	item is the dialog item number
*/
void
DTSDlgView::SetPopupText( int item, int menuItem, const char * text ) const
{
	// get the control handle
	if ( HIViewRef control = DTSDlgViewPriv::GetItem( this, item ) )
		{
		// extract the menu handle from the control
		// bail if there is none
		if ( MenuRef menu = GetControlPopupMenuHandle( control ) )
			{
			// insert any extra blank lines
			int count = GetNumPopupItems( item );
			for ( ;  count < menuItem;  ++count )
				AppendMenu( menu, "\p " );
			
			HIViewSetMaximum( control, count );
			
			// a NULL pointer is a separator line
			if ( not text )
				text = "-";
			
			// set the menu text
			if ( CFStringRef cs = CreateCFString( text ) )
				{
				__Verify_noErr( SetMenuItemTextWithCFString( menu, menuItem, cs ) );
				CFRelease( cs );
				}
			}
		}
}


/*
**	DTSDlgView::GetPopupText()
*/
void
DTSDlgView::GetPopupText( int item, int menuItem, char * text, size_t maxlen /* =256 */ ) const
{
	// get the control handle
	if ( HIViewRef control = DTSDlgViewPriv::GetItem( this, item ) )
		{
		// extract the menu handle from the control
		if ( MenuRef menu = ::GetControlPopupMenuHandle( control ) )
			{
			// get the item text
			CFStringRef cs;
			if ( noErr == CopyMenuItemTextAsCFString( menu, menuItem, &cs ) )
				{
				CFStringGetCString( cs, text, maxlen, kCFStringEncodingMacRoman );
				CFRelease( cs );
				return;
				}
			}
		}
	
	*text = '\0';
}


/*
**	DTSDlgView::GetNumPopupItems()
*/
int
DTSDlgView::GetNumPopupItems( int item ) const
{
	// get the control handle
	// extract the menu handle from the control
	// return its count
	// bail if anything's missing
	if ( HIViewRef control = DTSDlgViewPriv::GetItem( this, item ) )
		if ( MenuRef menu = GetControlPopupMenuHandle( control ) )
			return CountMenuItems( menu );
	
	return 0;
}


/*
**	DTSDlgView::GetWhere()
*/
void
DTSDlgView::GetWhere( DTSPoint * where ) const
{
	if ( not where )
		return;
	
	if ( DTSDlgViewPriv * dp = priv.p )
		*where = dp->dlgWhere;
}


/*
**	DTSDlgView::GetWhen()
*/
ulong
DTSDlgView::GetWhen() const
{
	const DTSDlgViewPriv * dp = priv.p;
	return dp ? dp->dlgWhen : 0;
}


/*
**	DTSDlgView::GetModifiers()
*/
uint
DTSDlgView::GetModifiers() const
{
	const DTSDlgViewPriv * dp = priv.p;
	return dp ? dp->dlgModifiers : 0;
}


/*
**	DTSDlgView::GetItemHit()
*/
int
DTSDlgView::GetItemHit() const
{
	const DTSDlgViewPriv * dp = priv.p;
	return dp ? dp->dlgItemHit : 0;
}


/*
**	DTSDlgView::Init()
*/
void
DTSDlgView::Init()
{
}


/*
**	DTSDlgView::Exit()
*/
void
DTSDlgView::Exit()
{
}


/*
**	DTSDlgView::Hit()
*/
void
DTSDlgView::Hit( int item )
{
	Dismiss( item );
}


/*
**	DTSDlgView::DrawItem()
*/
void
DTSDlgView::DrawItem( int /* item */, const DTSRect * /* bounds */ )
{
}


/*
**	DTSDlgView::KeyFilter()
*/
bool
DTSDlgView::KeyFilter( int /* item */, int /* ch */, uint /* mods */ )
{
	return false;
}


/*
**	DTS2Mac()
**
**	encapsulate the opacity of DialogRefs vis-a-vis WindowRefs and GrafPorts
*/
DialogRef
DTS2Mac( const DTSDlgView * dtsdlg )
{
	if ( not dtsdlg )
		return nullptr;
	
	DTSDlgViewPriv * p = dtsdlg->priv.p;
	return p ? p->dlgDialog : nullptr;
}


/*
**	HIViewRef	GetItem( const DTSDlgView * d, int item );
**
**	convenience method
*/
HIViewRef
GetItem( const DTSDlgView * d, int item )
{
//	HIViewRef result = DTSDlgViewPriv::GetItem( d, item, true );
	HIViewRef result = nullptr;
	if ( d )
		__Verify_noErr( GetDialogItemAsControl( DTS2Mac(d), item, &result ) );
	
	return result;
}


/*
**	DTSDlgView::HandleCarbonEvent()
**	carbon events
*/
OSStatus
DTSDlgViewPriv::HandleEvent( CarbonEvent& evt )
{
	OSStatus result = eventNotHandledErr;
	
	if ( DialogRef dlg = dlgDialog )
		{
		// for now we only redispatch scroll-wheel events,
		// and only to those controls that can put them to good use:
		// namely listboxes and databrowsers
		UInt32 kind = evt.Kind();
		
		if ( kEventClassMouse == evt.Class()
		&&   ( kEventMouseScroll == kind || kEventMouseWheelMoved == kind ) )
			{
			HIPoint pt2;
			OSStatus err = evt.GetParameter( kEventParamMouseLocation,
					typeHIPoint, sizeof pt2, &pt2 );
//			__Check_noErr( err );
			if ( noErr == err )
				{
				WindowRef w = GetDialogWindow( dlg );
				__Check( w );
#ifndef AVOID_QUICKDRAW
				Point pt = { short( pt2.y ), short( pt2.x ) };
				QDGlobalToLocalPoint( GetWindowPort( w ), &pt );
				
				ControlPartCode part( kControlNoPart );
				HIViewRef ctl = FindControlUnderMouse( pt, w, &part );
#else
				HIViewRef ctl = nullptr;
				HIPointConvert( &pt2, kHICoordSpace72DPIGlobal, nullptr, kHICoordSpaceWindow, w );
				__Verify_noErr( HIViewGetSubviewHit( HIViewGetRoot( w ), &pt2, true, &ctl ) );
				ControlPartCode part = kControlNoPart;
				__Verify_noErr( HIViewGetPartHit( ctl, &pt2, &part ) );
#endif  // AVOID_QUICKDRAW
				
				// it's a highly suspicious coincidence, is it not, that
				// kControlDataBrowserPart has the exact same value as kControlListBoxPart?
				if ( ctl && kControlDataBrowserPart == part )
					{
					// silly handsprings to avoid receiving the event
					// over and over again, recursively
					RemoveEventTypes( GetEventTypeCount( kScrollingEvents ), kScrollingEvents );
					
					result = evt.Send( GetControlEventTarget( ctl ), kEventTargetDontPropagate );
					
					AddEventTypes( GetEventTypeCount( kScrollingEvents ), kScrollingEvents );
					}
				}
			}
		}
	
	if ( eventNotHandledErr == result )
		result = CarbonEventResponder::HandleEvent( evt );
	
	return result;
}

#pragma mark -


/*
**	DTSDlgListPriv
*/
DTSDefineImplementFirmNoCopy(DTSDlgListPriv)


// this next definition cannot be visible until _after_ the DefineImplementXXX() macro above,
// otherwise you get a "specialization after instantiation" compiler error.

/*
**	DTSDlgList::~DTSDlgList()
**	destructor
*/
DTSDlgList::~DTSDlgList()
{
}


/*
**	DTSDlgListPriv::DTSDlgListPriv()
*/
DTSDlgListPriv::DTSDlgListPriv() :
		dlgNext( nullptr ),
		dlgHideLevel( 0 ),
		dlgView( nullptr ),
		dlgListHdl( nullptr ),
		dlgItem( 0 ),
		dlgDoubleClick( false ),
		dlgIsListControl( false )
{
	// install ourselves into the list
	dlgNext   = gRootList;
	gRootList = this;
}


/*
**	DTSDlgListPriv::~DTSDlgListPriv()
*/
DTSDlgListPriv::~DTSDlgListPriv()
{
	Exit();
}


/*
**	DTSDlgList::Init()
*/
void
DTSDlgList::Init( const DTSDlgView * dlg, int item )
{
	if ( not dlg )
		return;
	
	DTSDlgListPriv * p = priv.p;
	if ( not p )
		return;
	
	// save the DTS dialog
	p->dlgView = dlg;
	p->dlgItem = item;
	
	// get the Mac dialog
	DialogRef macDlg = DTS2Mac( dlg );
	
	// get the bounding box of the list
	DialogItemType kind;
	Handle hdl;
	Rect box;
	::GetDialogItem( macDlg, item, &kind, &hdl, &box );
	
	ListHandle list = nullptr;
	if ( kResourceControlDialogItem == kind )
		{
		// assume it's an LDEF
		HIViewRef ctrl = nullptr;
		OSStatus result = GetDialogItemAsControl( macDlg, item, &ctrl );
		if ( noErr == result  &&  ctrl )
			{
			Size junk;
			result = GetControlData( ctrl, kControlEntireControl,
							kControlListBoxListHandleTag,
							sizeof list, &list, &junk );
			__Check_noErr( result );
			}
		if ( noErr == result && list )
			{
			// make sure the size is right (a bug in OS X requires this workaround)
			Rect lBox = (*list)->rView;
			SInt16 cellHt = (*list)->cellSize.v;
			if ( 0 == cellHt )
				cellHt = 18;	// pick a number...
			SInt16 boxHt = lBox.bottom - lBox.top;
			SInt16 slop = boxHt - ((boxHt / cellHt) * cellHt);
			if ( slop != 0 )
				{
				// although we asked the list control to auto-size itself,
				// it didn't. (That's the X bug). So fix it up here. If we
				// don't do this, the list selection will frequently end up
				// being exactly one cell below the visible portion, thereby
				// confusing everyone.
				
				// first, resize the control itself. That'll probably mess up the
				// interior list widget even worse than it was before, but...
				SizeControl( ctrl, box.right - box.left, box.bottom - box.top - slop );
				
				// ... we can fix that in a jiffy. We have the technology!
				LSize( (*list)->rView.right - (*list)->rView.left, boxHt - slop, list );
				}
			
			// good to go! remember this fact for later
			p->dlgIsListControl = true;
			::SetKeyboardFocus( GetDialogWindow(macDlg), ctrl, kControlFocusNextPart );
			}
		}
	else
		{
		// set the list drawing procedure
		if ( not DTSDlgListPriv::gDrawListItemUPP )
			{
			DTSDlgListPriv::gDrawListItemUPP = NewUserItemUPP( DTSDlgListPriv::DrawListItem );
			__Check( DTSDlgListPriv::gDrawListItemUPP );
			}
		hdl = reinterpret_cast<Handle>( DTSDlgListPriv::gDrawListItemUPP );
		SetDialogItem( macDlg, item, kind, hdl, &box );
		}
		
	if ( not list )
		{
		// create the list
		box.top    +=  1;
		box.left   +=  1;	// ??? GetThemeMetric( kThemeMetricScrollBarOverlap )
		box.right  -= 16;	// ??? GetThemeMetric( kThemeMetricScrollBarWidth )
		box.bottom -=  1;
		
		const Rect dataBounds = { 0, 0, 0, 1 };

		Point cSize;
		cSize.h = box.right - box.left;
		cSize.v = 18;
		
		list = LNew( &box,			// view bounds
					&dataBounds,	// data bounds
					cSize,			// cell size
					0,				// proc id
					GetDialogWindow( macDlg ),	// window
					true,			// draw it flag
					false,			// grow icon flag
					false,			// horz scroll bar flag
					true );			// vert scroll bar flag
		CheckPointer( list );
		assert( list );
		}
	
	(*list)->selFlags = lOnlyOne; // only allow one item to be selected
	
	p->dlgListHdl = list;
}


/*
**	DTSDlgList::Exit()
*/
void
DTSDlgList::Exit()
{
	if ( DTSDlgListPriv * p = priv.p )
		p->Exit();
}


/*
**	DTSDlgListPriv::Exit()
*/
void
DTSDlgListPriv::Exit()
{
	// remove ourselves from the list
	DTSDlgListPriv * test;
	for ( DTSDlgListPriv ** link = &gRootList; (test = *link) != nullptr;  link = &test->dlgNext )
		{
		if ( test == this )
			{
			*link = test->dlgNext;
			break;
			}
		}
	
	// dispose of the list handle, but only if we made it ourselves.
	// AppearanceMgr's list-controls magically dispose themselves at DisposeDialog() time
	if ( not dlgIsListControl && dlgListHdl )
		LDispose( dlgListHdl );
	
	// clear fields
	dlgNext    = nullptr;
	dlgListHdl = nullptr;
}


/*
**	DTSDlgList::SetText()
*/
void
DTSDlgList::SetText( int item, const char * text ) const
{
	DTSDlgListPriv * p = priv.p;
	if ( not p )
		return;
	
	ListHandle list = p->dlgListHdl;
	if ( not list )
		return;
	
	// turn off drawing when adding a row
	// the list will be drawn when the text is set
	// (according to eric)
	ListBounds lBounds;
	GetListDataBounds( list, &lBounds );
	int last = lBounds.bottom;
	if ( item == last )
		{
		HideDrawing();
		LAddRow( 1, 0x7FFF, list );
		ShowDrawing();
		}
	
	int dataLen = strlen( text ) + 1;
	Point cell;
	cell.h = 0;
	cell.v = item;
	LSetCell( text, dataLen, cell, list );
	
	// I suspect this is redundant for true list controls
	if ( not p->dlgIsListControl && p->IsDrawingEnabled() )
		{
		HIViewRef ctl = nullptr;
		OSStatus result = -1;
		DialogRef macdlg = DTS2Mac( p->dlgView );
		if ( macdlg )
			result = GetDialogItemAsControl( macdlg, p->dlgItem, &ctl );
		if ( noErr == result && ctl )
			Draw1Control( ctl );
		}
}


/*
**	DTSDlgList::GetText()
*/
void
DTSDlgList::GetText( int item, char * text, size_t maxlen /* =256 */ ) const
{
	DTSDlgListPriv * p = priv.p;
	if ( not p )
		return;
	
	ListHandle list = p->dlgListHdl;
	if ( not list )
		return;
	
	short dataLen = 0x7FFF;
	if ( maxlen < static_cast<uint>( dataLen ) )
		dataLen = maxlen;
	Point cell;
	cell.h = 0;
	cell.v = item;
	LGetCell( text, &dataLen, cell, list );
}


/*
**	DTSDlgList::SetSelect()
*/
void
DTSDlgList::SetSelect( int item ) const
{
	DTSDlgListPriv * p = priv.p;
	if ( not p )
		return;
	
	ListHandle list = p->dlgListHdl;
	if ( not list )
		return;
	
	DTSPoint cell( 0, item );
	LSetSelect( true, *DTS2Mac( &cell ), list );
	
	// scroll it into view
	// (this is a wee bit slicker than LAutoScroll)
	DTSRect vis;
	GetListVisibleCells( list, DTS2Mac( &vis ) );
	
	// don't bother if new cell is already visible
	if ( not cell.InRect( &vis ) )
		{
		int delta = 0;
		if ( cell.ptV > vis.rectBottom - 1 )
			delta = cell.ptV - vis.rectBottom + 1;		// move up
		else
		if ( cell.ptV < vis.rectTop )
			delta = cell.ptV - vis.rectTop;				// move down
		
		LScroll( 0, delta, list );
		}
}


/*
**	DTSDlgList::GetSelect()
*/
int
DTSDlgList::GetSelect() const
{
	const DTSDlgListPriv * p = priv.p;
	int select =  p ? p->GetSelect() : 0;
	return select;
}


/*
**	DTSDlgListPriv::GetSelect()
*/
int
DTSDlgListPriv::GetSelect() const
{
	ListHandle list = dlgListHdl;
	if ( not list )
		return 0;
	
	Point cell = { 0, 0 };
	int item = -1;
	if ( LGetSelect( true, &cell, list ) )
		item = cell.v;
	
	return item;
}


/*
**	DTSDlgList::KeyStroke()
*/
bool
DTSDlgList::KeyStroke( int ch, uint /*modifiers*/ )
{
	const DTSDlgListPriv * p = priv.p;
	if ( not p )
		return false;
	
	if ( p->dlgIsListControl )
		{
		// we can ignore it; the control will do all the work
		return false;
		}
	
	ListHandle list = p->dlgListHdl;
	if ( not list )
		return false;
	
	int selCell = p->GetSelect();
	
	// determine list limits
	ListBounds b;
	::GetListDataBounds( list, &b );
	int maxCells = b.bottom - 1;
	
	::GetListVisibleCells( list, &b );
	int cellsVis = b.bottom - b.top;
	
	switch ( ch )
		{
		case kUpArrowKey:
		case kLeftArrowKey:
			if ( -1 == selCell )
				selCell = 0;
			else
			if ( 0 == selCell )
				; // nothing
			else
				--selCell;
			break;
		
		case kDownArrowKey:
		case kRightArrowKey:
			if ( -1 == selCell )
				selCell = maxCells;
			else
			if ( maxCells == selCell )
				; // nothing
			else
				++selCell;
			break;
		
		case kHomeKey:
			if ( -1 != selCell )
				selCell = 0;
			break;
		
		case kEndKey:
			if ( -1 != selCell )
				selCell = maxCells;
			break;
			
		case kPageUpKey:
			LScroll( 0, -(cellsVis - 1), list );
			return true;
			
		case kPageDownKey:
			LScroll( 0, cellsVis - 1, list );
			return true;
		
		default:
			// we don't handle other keys yet
			return false;
		}
	
	// only keys we do handle get through to here
	if ( -1 != selCell )
		{
		// first, we must de-select
		int prevSelect = p->GetSelect();
		if ( -1 != prevSelect )
			{
			Cell c;
			c.h = 0;
			c.v = prevSelect;
			LSetSelect( false, c, list );
			}
		// select the new
		SetSelect( selCell );
		}
	return true;
}


/*
**	DTSDlgList::Deselect()
*/
void
DTSDlgList::Deselect( int item ) const
{
	const DTSDlgListPriv * p = priv.p;
	if ( not p )
		return;
	
	ListHandle list = p->dlgListHdl;
	if ( not list )
		return;
	
	Point cell;
	cell.h = 0;
	cell.v = item;
	LSetSelect( false, cell, list );
}


/*
**	DTSDlgListPriv::Click()
*/
void
DTSDlgListPriv::Click()
{
	if ( dlgIsListControl )
		{
		HIViewRef ctl = nullptr;
		OSStatus result = paramErr;
		if ( DialogRef macdlg = DTS2Mac( dlgView ) )
			result = GetDialogItemAsControl( macdlg, dlgItem, &ctl );
		if ( noErr == result && ctl )
			{
			Boolean wasClick = false;
			Size junk;
			result = GetControlData( ctl, kControlEntireControl,
							kControlListBoxDoubleClickTag,
							sizeof wasClick, &wasClick, &junk );
			if ( noErr == result )
				{
				dlgDoubleClick = wasClick && ( GetSelect() != -1 );
				return;
				}
			}
		}
	
	ListHandle list = dlgListHdl;
	if ( not list )
		return;
	
	DTSPoint where( 0, 0 );
	dlgView->GetWhere( &where );
	dlgDoubleClick = LClick( *DTS2Mac(&where), 0, list );
}


/*
**	DTSDlgList::Reset()
**
**	delete all items in the list
*/
void
DTSDlgList::Reset()
{
	const DTSDlgListPriv * p = priv.p;
	if ( not p )
		return;
	
	ListHandle list = p->dlgListHdl;
	if ( not list )
		return;
	
	LDelColumn( 0, 0, list );
	LDelRow( 0, 0, list );
	LAddColumn( 1, 0, list );
}


/*
**	DTSDlgList::Remove()
**
**	remove one item
*/
DTSError
DTSDlgList::Remove( int item )
{
	const DTSDlgListPriv * p = priv.p;
	if ( not p )
		return -1;
	
	ListHandle list = p->dlgListHdl;
	if ( not list )
		{
		// no list
		return -1;
		}
	
	ListBounds bounds;
	GetListDataBounds( list, &bounds );
	
	if ( item < bounds.top
	||	 item >= bounds.bottom )
		{
		// out of range
		return paramErr;
		}
	
	// delete 1 row starting at item
	LDelRow( 1, item, list );
	return ::MemError();
}


/*
**	DTSDlgList::WasDoubleClick()
*/
bool
DTSDlgList::WasDoubleClick() const
{
	const DTSDlgListPriv * p = priv.p;
	return p ? p->dlgDoubleClick : false;
}


/*
**	DTSDlgList::HideDrawing()
**	suppress drawing of list contents
**	These act sort of like HidePen/ShowPen, in that nesting is allowed,
**	provided that each Hide() must eventually be balanced by a Show()
**	(but it's safe to have an excess number of Show()'s)
*/
void
DTSDlgList::HideDrawing() const
{
	if ( DTSDlgListPriv * p = priv.p )
		p->HideDrawing();
}


/*
**	DTSDlgList::ShowDrawing()
**	resume drawing
*/
void
DTSDlgList::ShowDrawing() const
{
	if ( DTSDlgListPriv * p = priv.p )
		p->ShowDrawing();
}


/*
**	DTSDlgListPriv::HideDrawing()
**	suspend drawing
*/
void
DTSDlgListPriv::HideDrawing()
{
	// no drawing when level >= 1
	if ( 1 == ++dlgHideLevel )
		LSetDrawingMode( false, dlgListHdl );
}


/*
**	DTSDlgListPriv::ShowDrawing()
**	resume drawing
*/
void
DTSDlgListPriv::ShowDrawing()
{
	// resume drawing when level returns to 0
	int hideLvl = dlgHideLevel - 1;
	if ( 0 == hideLvl )
		LSetDrawingMode( true, dlgListHdl );
	
	// but don't lower the level below 0
	if ( hideLvl >= 0 )
		dlgHideLevel = hideLvl;
}


/*
**	DrawListItem	[static]
*/
void
DTSDlgListPriv::DrawListItem( DialogRef dlg, DialogItemIndex item )
{
	// find the dialog list in the linked list
	DTSDlgListPriv * dtsList = FindList( dlg, item );
	if ( not dtsList )
		return;
	
	// draw the bounding box
	DTSRect box;
	DialogItemType kind;
	Handle hdl;
	GetDialogItem( dlg, item, &kind, &hdl, DTS2Mac(&box) );
	
	{
	DTSRect b = box;
	b.Inset( 1, 1 );	// kThemeMetricListBoxFrameOutset?
	ThemeDrawState state = kThemeStateActive;
	if ( not IsWindowActive( GetDialogWindow( dlg ) )
	||   gSuspended )
		{
		state = kThemeStateInactive;
		}
	/* OSStatus err = */ DrawThemeListBoxFrame( DTS2Mac( &b ), state );
	}
	
	// draw the list
#ifdef AVOID_QUICKDRAW
	LUpdate( nullptr, dtsList->dlgListHdl );
#else
	DTSRegion rgn;
	rgn.Set( &box );
	LUpdate( DTS2Mac(&rgn), dtsList->dlgListHdl );
#endif
}


/*
**	FindList()		[static]
*/
DTSDlgListPriv *
DTSDlgListPriv::FindList( DialogRef dlg, int item )
{
	for ( DTSDlgListPriv * dtsList = gRootList;  dtsList;  dtsList = dtsList->dlgNext )
		{
		if ( DTS2Mac(dtsList->dlgView) == dlg
		&& 	dtsList->dlgItem == item )
			{
			return dtsList;
			}
		}
	
	return nullptr;
}

#pragma mark -


/*
**	GenericError()
**
**	put a generic error dialog box
*/
void
GenericError( const char * format, ... )
{
	va_list params;
	va_start( params, format );
	(void) VGenericDialog( kGDError, format, params );
	va_end( params );
}


/*
**	GenericOkCancel()
**
**	put a generic ok cancel dialog box
*/
int
GenericOkCancel( const char * format, ... )
{
	va_list params;
	va_start( params, format );
	int buttonHit = VGenericDialog( kGDOkCancel, format, params );
	va_end( params );
	
	return ( kAlertStdAlertOKButton == buttonHit ) ? kGenericOk : kGenericCancel;
}


/*
**	GenericYesNoCancel()
**
**	put a generic yes no cancel dialog box
*/
int
GenericYesNoCancel( const char * format, ... )
{
	va_list params;
	va_start( params, format );
	int buttonHit = VGenericDialog( kGDYesNoCancel, format, params );
	va_end( params );
	
	if ( kAlertStdAlertOKButton == buttonHit )
		return kGenericYes;
	if ( kAlertStdAlertOtherButton == buttonHit )
		return kGenericNo;
	return kGenericCancel;
}



/*
**	SetupAlertRecord()
**
**	helper for VGenericDialog()
**	does some basic customizations of the alert record
*/
static AlertType
SetupAlertRecord( GDKind kind, AlertStdCFStringAlertParamRec& rec )
{
	rec.movable = true;
	rec.cancelText = (CFStringRef) kAlertDefaultCancelText;
	rec.cancelButton = kAlertStdAlertCancelButton;
	rec.position = kWindowAlertPositionMainScreen;
	
	// now customize the alert, based on desired kind
	AlertType alertKind = kAlertNoteAlert;
	switch ( kind )
		{
		case kGDError:
			alertKind = kAlertCautionAlert;		// use caution alert
			rec.cancelButton = 0;				// no cancel button
			rec.cancelText = nullptr;			// thus no text for it
			break;
		
		case kGDOkCancel:
			break;								// use default, as-is
		
		case kGDYesNoCancel:
				// TODO: use CFCopyLocalizedString?
			rec.defaultText = CFSTR("Yes");		// OK button text
			rec.otherText   = CFSTR("No");		// other (3rd) button text
			break;
		}
	
	return alertKind;
}


/*
**	VGenericDialog()
**
**	put up a generic dialog box.
**	Does all the real work of the three GenericXXX() functions above.
*/
int
VGenericDialog( GDKind kind, const char * format, va_list params )
{
	DTSShowArrowCursor();
	
	// produce formatted C-string
	char * buff = nullptr;
	int bflen = vasprintf( &buff, format, params );
	if ( -1 == bflen )  // paranoia
		return kGenericCancel;		// what else can we do?
	
	DialogItemIndex buttonHit = kAlertStdAlertCancelButton;
	
	// slightly more modern [than what used to be here] way, with CFStrings
	AlertStdCFStringAlertParamRec rec;
	OSStatus err = GetStandardAlertDefaultParams( &rec, kStdCFStringAlertVersionOne );
	__Check_noErr( err );
	if ( noErr == err )
		{
		AlertType alertKind = SetupAlertRecord( kind, rec );
		
		// break up multi-line messages into title and body
		CFStringRef bodyMsg = nullptr;
		
		// trim trailing whitespace
		char * end = buff + bflen - 1;
		while ( isspace( * (uchar *) end ) )
			{
			*end-- = '\0';
			--bflen;
			}
		
		// skip leading whitespace
		const char * start = buff;
		while ( isspace( * (const uchar *) start )  )
			{
			++start;
			--bflen;
			}
		
		// find first "real" NL, if any
		if ( const char * p = strpbrk( start, "\r\n" ) )
			{
			int headLen = p - start;		// omits the NL
			int tailLen = bflen - headLen;
			
			// skip the linebreak, and any adjacent, consecutive, succeeding whitespace
			while ( isspace( * (const uchar *) p ) )
				{
				++p;
				--tailLen;
				}
			
			// everything thereafter constitutes the body message
			if ( tailLen )
				{
				bodyMsg = CFStringCreateWithBytes( kCFAllocatorDefault,
							reinterpret_cast<const uchar *>( p ), tailLen,
							kCFStringEncodingMacRoman, false );
				}
			
			bflen = headLen;
			}
		
		// the title portion is whatever's left (which could be the entire message,
		// if there weren't any newlines)
		CFStringRef titleMsg = CFStringCreateWithBytes( kCFAllocatorDefault,
						reinterpret_cast<const uchar *>( start ), bflen,
						kCFStringEncodingMacRoman, false );
		
		// fail if we couldn't make at least one message
		if ( titleMsg || bodyMsg )
			{
			DialogRef dlg;
			err = CreateStandardAlert( alertKind, titleMsg, bodyMsg, &rec, &dlg );
			__Check_noErr( err );
			if ( noErr == err )
				{
				err = RunStandardAlert( dlg, nullptr, &buttonHit );
				__Check_noErr( err );
				}
			}
		
		// cleanup
		if ( titleMsg )
			CFRelease( titleMsg );
		if ( bodyMsg )
			CFRelease( bodyMsg );
		}
	
	if ( buff )
		::free( buff );		// from vasprintf()
	
	return buttonHit;
}

