/*
 *  Dialog_mach.h
 *  Created on 10/29/07.
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

#ifndef DIALOG_MACH_H
#define DIALOG_MACH_H


enum	// text field event notifications
{
		// don't want any notifications
	kTFNotifyNone			= 0U,
	
		// field loses focus and contents are different
		// (see kEventClassTextField/kEventTextAccepted)
	kTFNotifyAccept			= 1U << 0,
		
		// field contents may change
		// (see kEventClassTextField/kEventTextShouldChangeInRange)
	kTFNotifyShouldChange	= 1U << 1,
	
		// field contents have changed
		// (see kEventClassTextField/kEventTextDidChange)
	kTFNotifyDidChange		= 1U << 2
};
typedef UInt32 TFNotification;


/*
**	class HIDialog
**
**	a new, more-modern replacement for DTSDlgView.
**	the semantics are broadly similar but the details are of course quite different.
*/

/*
	This is a work in progress, and I'm vacillating about some of the design decisions.
	
	The notion of using HIViewIDs to identify dialog items turns out to be a little bit
	unwieldy in practice. It might be better for each dialog class to furnish its own
	unique signature OSType, and then refer to items by number only.
	A utility in this parent class would then invisibly combine the signature and item-numbers
	into HIViewIDs to pass on to the HIToolbox. (See the plethora of Item() routines.)
	
	It's mildly annoying that, frequently, Exit() and the caller of Run() _both_ need
	to look at the result of Dismissal().
	
	There's no good mechanism for a sheet-based dialog to notify its parent that it's
	been dismissed, and that the parent may need to update its own model.
	
	Need to support the old 'DlgK' mechanism, or something like it.
	
	It's quixotic (at best; a better word might be "boneheaded") to be expending energy
	on new Carbon-based code, when the sensible thing would be to rewrite it all for Cocoa.
*/
class HIDialog : public HICommandResponder
{
	typedef HICommandResponder SUPER;
	
public:
				HIDialog() :
					mWindow( nullptr ),
					mParent( nullptr ),
					mReason( 0 ),
					mLastCursor( UINT_MAX )	// not a valid cursorID
					{}
				
	virtual		~HIDialog() { TearDown(); }
				
				// subclasses must provide the name of their nib file...
	virtual	CFStringRef		GetNibFileName() const = 0;
				// ... and the name of their window, therein
				// (but in the common case where the two are the same, you can
				// refrain from implementing GetWindowNibName())
	virtual CFStringRef		GetNibWindowName() const;
	
				// NB! if parentWin is non-NULL, the dialog is run as a sheet.
				// If the window class (as set in the nib file) isn't compatible,
				// Bad Things will happen.
	OSStatus	Run( WindowRef parentWin = nullptr );
	
				// make the dialog go away, and stash the commandID to be retrieved later
				// by Dismissal().  Typical commandIDs will be kHICommandOK/Cancel.
				// (But see Validate() below for an important caveat!)
	void		Dismiss( UInt32 commandID );
	
				// returns the commandID that was passed to Dismiss()
	UInt32		Dismissal() const { return mReason; }
	
				// fundamental accessor for dialog items
	HIViewRef	GetItem( const HIViewID& itemID ) const;
	
				// applicable to just about any type of item
	OSStatus	SetVisible( const HIViewID& itemID, bool vis ) const;
	bool		IsItemVisible( const HIViewID& itemID ) const;
	
				// these are for enabling/disabling items. So far I haven't needed a way
				// to interface with HIViewSetActivated()/HIViewIsActive(), or hiliting.
	OSStatus	AbleItem( const HIViewID& itemID, bool enable ) const;
	bool		IsItemEnabled( const HIViewID& itemID ) const;
	
				// these two treat the item as having a numeric value
	int			GetValue( const HIViewID& itemID ) const;
	OSStatus	SetValue( const HIViewID& itemID, int value ) const;
				
				// these only make sense on text-type items
	CFStringRef	CopyText( const HIViewID& itemID ) const;
	bool		GetText( const HIViewID& itemID, char * buff, size_t bufsz,
							CFStringEncoding enc = kCFStringEncodingMacRoman ) const;
	OSStatus	SetText( const HIViewID& itemID, CFStringRef text ) const;
	OSStatus	SetText( const HIViewID& itemID, const char * text ) const;	// legacy; MacRoman
	OSStatus	SelectText( const HIViewID& itemID, short b = 0, short e = 0x7FFF ) const;
	
				// these two are for text fields _containing_ numeric values (as strings):
				// SetNumber() basically calls printf("%d")
	OSStatus	SetNumber( const HIViewID& itemID, int value ) const;
				// uses CFStringGetIntValue(), so is pretty tolerant of non-numeric text.
				// But if you supply the optional 2nd param, we'll tell you whether or not
				// the text was a well-formed number.
	int			GetNumber( const HIViewID& itemID, bool * strictlyValid = nullptr ) const;
	
				// exactly like DTSDlgView version
	OSStatus	SetFloat( const HIViewID& itemID, double value ) const;
	double		GetFloat( const HIViewID& itemID ) const;
	
#if MAC_OS_X_VERSION_MAX_ALLOWED > 1040
				// iconID must denote one of the preregistered constants, from ResourceIDs.h
				// (if you want more flexibility, go directly to HIViewSetImageContent())
	OSStatus	SetIcon( const HIViewID& itemID, OSType iconID ) const;
#endif
	
				// for popup menu items
	int			CountPopupMenuItems( const HIViewID& itemID ) const;
	void		SetPopupText( const HIViewID& itemID, int menuItem, CFStringRef text ) const;
	CFStringRef	CopyPopupText( const HIViewID& itemID, int menuItem ) const;
	
				// to change the dialog window's title
	OSStatus	SetTitle( CFStringRef wTitle ) const;	// not for sheets -- duh
	OSStatus	SetTitle( const char * wTitle, CFStringEncoding encoding = 0 ) const;	// MacRoman
	
				// enroll for text-change notifications
	OSStatus	SetFieldNotifications( const HIViewID& fieldID, TFNotification notifs );
	
#if MAC_OS_X_VERSION_MAX_ALLOWED > 1040
				// install a length-limit restriction on a text-input field
	OSStatus	SetFieldLengthLimit( const HIViewID& fieldID, UInt32 maxLength );
#endif
	
protected:
	virtual void	Init() {}		// subclasses can twiddle before we run
	virtual void	Exit() {}		// ditto before going away
	
					// clicking OK will _NOT_ dismiss the dialog until this returns true
	virtual bool	Validate() const { return true; }
	
						// if you've enrolled a text item for notifications (TFNotification),
						// you must implement the corresponding one (or more) of these.
	virtual bool		FieldDidAcceptText( HIViewRef )		{ return false; }
	virtual OSStatus	FieldShouldChangeText( HIViewRef, const CarbonEvent& )
							{ return eventNotHandledErr; }
	virtual bool		FieldDidChangeText( HIViewRef )		{ return false; }
	
						// casts a HIDialog* to a WindowRef (handy for presenting
						// sub-dialogs as sheets)
	operator			WindowRef() const { return mWindow; }
	WindowRef			GetParent() const { return mParent; }
	
	// normally, Run() causes the focus to advance to the first available view.
	// If subclasses implement this to return false, that behavior is skipped.
	virtual bool		WantAutoFocusAdvance() const { return true; }
	
	
	// if your window is not a sheet, and your override of this method returns a valid
	// string, then we'll remember your window's position, and restore it the next time.
	virtual CFStringRef	GetPositionPrefKey() const { return nullptr; }
	
						// subclasses that want to handle button clicks and other command-type
						// events should implement this, returning true _only_ for commands
						// they handle themselves.
	virtual bool		DoCommand( const HICommandExtended& ) { return false; }
	
						// slightly more general version of DoCommand()
	virtual OSStatus	ProcessCommand( const HICommandExtended& cmd, CarbonEvent& );
						
						// unlikely to be useful, except perhaps for popup menu items?
	virtual OSStatus	UpdateCommandStatus( const HICommandExtended& cmd, CarbonEvent& evt,
								HICommandResponder::EState& oState, CFStringRef& oText ) const
							{
//							return eventNotHandledErr;
							return SUPER::UpdateCommandStatus( cmd, evt, oState, oText );
							}
						
						// basic event funnel. Implement this if you have special needs
	virtual OSStatus	HandleEvent( CarbonEvent& );
	
private:
	OSStatus			TearDown();
	OSStatus			HandleTextFieldEvent( HIViewRef, const CarbonEvent& );
	void				DoCursorChange( const HIPoint& mouseLoc, HIViewRef ) const;
	static OSStatus		DoViewEvent( EventHandlerCallRef, EventRef, void * );
	
	// our data
	WindowRef		mWindow;		// our modal or sheet window
	WindowRef		mParent;		// NULL if not a sheet
	UInt32			mReason;		// how we got Dismiss()'d
	mutable ThemeCursor	mLastCursor;	// for cursor tracking
};

#endif  // DIALOG_MACH_H

