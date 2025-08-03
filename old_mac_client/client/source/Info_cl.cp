/*
**	Info_cl.cp		ClanLord
**
**	Copyright 2023 Delta Tao Software, Inc.
** 
**	Licensed under the Apache License, Version 2.0 (the "License");
**	you may not use this file except in compliance with the License.
**	You may obtain a copy of the License at
** 
**     https://www.apache.org/licenses/LICENSE-2.0
** 
**	Unless required by applicable law or agreed to in writing, software
**	distributed under the License is distributed on an "AS IS" BASIS,
**	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
**	See the License for the specific language governing permissions and
**	imitations under the License.
*/

//#pragma GCC diagnostic warning "-Wdeprecated-declarations"

#include "ClanLord.h"
#include "CommandIDs_cl.h"
#include "Dialog_mach.h"
#include "GameTickler.h"
#include "BrowserList.h"
#include "KeychainUtils.h"


/*
**	Entry Routines
*/
/*
DTSError	CharacterManager();
DTSError	SelectCharacter();
DTSError	EnterPassword();
DTSError	GetHostAddr();
*/


/*
**	Internal Definitions
*/
// port number to use, if none specified by user
#define DEFAULT_PORT	":5010"
#define TEST_PORT		":5011"

//#define kSelfAddress	"localhost" DEFAULT_PORT
#define kSelfAddress	"127.0.0.1" DEFAULT_PORT

#ifdef DEBUG_VERSION
# ifdef ARINDAL
#  define	kTestServerAddress	"server.arindal.com" TEST_PORT
# else
			// old test host, before 18 Aug 2004
//#  define	kTestServerAddress	"server.clanlord DOT com" TEST_PORT	
			// DNS change 27 July 2014
#  define	kTestServerAddress	"server.deltatao.com" TEST_PORT
# endif
#endif

// does the CharMgr dialog display account paid-up dates?
// (which don't mean a lot under the free-to-play regime)
#define DISPLAY_PAID_UP_DATES	0


/*
**	class RestrictEdit
**
**	Wrapper for an edit-text control in NIB-based dialogs & HIViews.
**
**	This class requires that the underlying input field be a modern
**	EditUnicodeText control, not an old-fashioned EditText one. The E-U-T
**	control can be either the plain or the password variant -- we don't
**	care.
**	
**	The warning icon (if there is one) will be shown or hidden dynamically,
**	depending on whether the field's contents are fit for purpose (as
**	determined by the Acceptable() method, which subclasses should
**	override).
*/
class RestrictEdit
{
protected:
	HIViewRef			reControl;			// the underlying edit-text control
	HIViewRef			reWarningIcon;		// can be NULL
	CFCharacterSetRef	reInvalidChars;		// characters to be filtered out
	CFIndex				reMaxLen;			// length limit
	
public:
					RestrictEdit() :
						reControl( nullptr ),
						reWarningIcon( nullptr ),
						reInvalidChars( nullptr ),
						reMaxLen( 0 )
						{}
					
					~RestrictEdit()
						{
						if ( reInvalidChars )
							CFRelease( reInvalidChars );
						}
	
	void			Init( HIViewRef item, CFStringRef validChars,
							CFIndex maxLen, const char * initialText,
							HIViewRef icon = nullptr )
						__attribute__(( __nonnull__( 2, 3 ) ));
	
	void			GetText( size_t dstLen, char * dst,
						CFStringEncoding enc = kCFStringEncodingMacRoman ) const
						__attribute__(( __nonnull__ ));
	
	void			SelectText( int start = 0, int end = 0x7FFF ) const;
	
	void			SetText( const char * newText ) const
						__attribute__(( __nonnull__ ));
	
private:
	CFStringRef		CreateCleanCopy( CFStringRef candidate ) const;
	OSStatus		ShouldChangeText( HIViewRef vu, const CarbonEvent& evt ) const;
	void			DidChangeText() const;
	
					// is text valid for intended use?
	virtual bool	Acceptable() const { return true; }
	
	static OSStatus	HandleTextEvent( EventHandlerCallRef, EventRef, void * );
};


/*
**	specialization for passwords
*/
class PasswordEdit : public RestrictEdit
{
public:
	void			Init( HIViewRef item,
						HIViewRef icon, const char * initialText );
	
					// is the current contents OK as a CL password?
	virtual bool	Acceptable() const;
};


/*
**	ditto, for charnames
*/
class NameEdit : public RestrictEdit
{
public:
	void			Init( HIViewRef item, HIViewRef icon, const char * initText );
	
					// is the current contents OK as a name?
	virtual bool	Acceptable() const;
};


/*
**	class SelectAccountDlg
**
**	asks user to specify his/her account name & password,
**	prior to presenting the Character Manager dialog
*/
class SelectAccountDlg : public HIDialog
{
	static const OSType Signature = 'SAct';
	static HIViewID Item( int n ) { return (HIViewID) { Signature, n }; }
	
	virtual CFStringRef	GetNibFileName() const
				{ return CFSTR("Select Account"); }
//	virtual CFStringRef	GetPositionPrefKey() const
//				{ return CFSTR("SelectAccountDlgPosition"); }
	
private:
	RestrictEdit		sadAcctName;
	PasswordEdit		sadAcctPass;
	
	virtual void	Init();
	virtual bool	DoCommand( const HICommandExtended& );
	virtual bool	Validate() const;
	void			SaveInfo() const;
	
	// relookup passwords whenever the account-name field changes
	virtual bool	FieldDidChangeText( HIViewRef vu );
	
public:
	enum
		{
		sadAcctNameEdit = 1,
		sadAcctPassEdit = 2,
		sadSavePassBox = 3,
		sadDemoText = 4,
		sadDemoBtn = 5,
		sadPassIcon = 6,
		};
	enum
		{
		cmd_TryDemo = 'Demo'
		};
};


class CharacterManagerDlg;	// forward

/*
**	class CharacterList
**	databrowser for character names attached to an account
*/
class CharacterList : public BrowserList
{
	static const DataBrowserPropertyID prop_CharName = 'Name';
	
	virtual OSStatus	GetItemData( DataBrowserItemID,
							DataBrowserPropertyID, DataBrowserItemDataRef );
	
	virtual void		DoSelectionSetChanged( DataBrowserItemID,
							DataBrowserItemDataRef );
	
	virtual void		DoDoubleClick( DataBrowserItemID item,
							DataBrowserItemDataRef data );
	
	static OSStatus		DoEvent( EventHandlerCallRef, EventRef, void * );
	OSStatus			HandleEvent( CarbonEvent& );
	
	CharacterManagerDlg *	mParent;
	CFArrayRef				mChars;
	
public:
						CharacterList() :
							mParent( nullptr ),
							mChars( nullptr )
							{}
						~CharacterList()
							{
							if ( mChars )
								CFRelease( mChars );
							}
	
	virtual OSStatus	Init( CharacterManagerDlg * parent, HIViewRef vu );
	OSStatus			PopulateList();
	void				ParseLogMessage( const Message& msg,
							int& ourIdx, int& maxChars );
	int					CountCharacters() const;
	CFStringRef			GetNthCharName( int n ) const;
};


/*
**	class CharacterManagerDlg
**
**	runs the Character Manager dialog
*/
class CharacterManagerDlg : public HIDialog, public GameTickler
{
	static const OSType Signature = 'CMgr';
	static HIViewID Item( int n ) { return (HIViewID) { Signature, n }; }
	virtual CFStringRef GetNibFileName() const
				{ return CFSTR("Character Manager"); }
	virtual CFStringRef	GetPositionPrefKey() const
				{ return CFSTR("CharacterManagerDlgPosition"); }
	
	Message			cmdMsg;
	CharacterList	cmdCharList;
	bool			cmdIsDemo;
	
public:
						CharacterManagerDlg() :
							cmdIsDemo( false )
							{}
						~CharacterManagerDlg() {}
	
	HIViewRef			GetSpinnerView() const
							{
							return GetItem( Item( cmdSpinner ) );
							}
	
	enum
		{
		cmdDone = 7,		// can have OK &| Cancel roles (but always reads "Cancel")
		cmdAccountText = 8,
		cmdCharacterList = 1,
		cmdFreeText = 6,
		cmdPlayBtn = 2,
		cmdCreateBtn = 3,
		cmdDeleteBtn = 4,
		cmdPasswordBtn = 5,
		cmdSpinner = 9,
		};
	enum
		{
		cmd_SelectCharacter = 'SChr',		// from the databrowser
		cmd_PlayChar		= 'Play',
		cmd_CreateChar		= kHICommandNew,
		cmd_DeleteChar		= 'Dele',
		cmd_Password		= 'Pwd!',
		};
	
private:	
	virtual void	Init();
	virtual void	Exit();
	virtual bool	DoCommand( const HICommandExtended& );
	
	void		InitCharList();
	void		SaveInfo() const;
	void		DoCreate();
	DTSError	DoCreateLoop( const char * charpass );
	DTSError	DoTransferPass();
	void		DoDelete();
	void		DoPassword();
	DTSError	GetSelectedName( char * dst, size_t dstLen ) const;
	void		UpdateButtons() const;
	
	friend class CharacterList;

public:
	// accessors
	void		SetIsDemo( bool bIsDemo ) { cmdIsDemo = bIsDemo; }
	Message *	GetMessage() { return &cmdMsg; }
};


/*
**	class CreateCharacterDlg
**
**	runs the character-creation dialog
*/
class CreateCharacterDlg : public HIDialog
{
	static const OSType Signature = 'Char';
	static HIViewID Item( int n ) { return (HIViewID) { Signature, n }; }
	virtual CFStringRef GetNibFileName() const
				{ return CFSTR("Create Character"); }
//	virtual CFStringRef	GetPositionPrefKey() const
//				{ return CFSTR("CreateCharacterDlgPosition"); }
	
	NameEdit		ccdCharName;
	
	virtual void	Init();
	virtual bool	DoCommand( const HICommandExtended& );
	virtual bool	Validate() const;
	
	void			SaveInfo() const;
	
public:
	enum
		{
		ccdCharNameEdit = 1,
		ccdBadNameIcon = 2,
		};
	enum { cmd_NamePolicy = 'Plcy' };
};


/*
**	class DeleteCharacterDlg
**
**	requests confirmation before deleting the currently
**	selected character, via CharMgr
*/
class DeleteCharacterDlg : public HIDialog
{
	static const OSType Signature = 'DelC';
	static HIViewID Item( int n ) { return (HIViewID) { Signature, n }; }
	virtual CFStringRef GetNibFileName() const
				{ return CFSTR("Delete Character"); }
	
private:
	// overloaded routines
	virtual void	Init();
	
public:
	enum
		{
		dcdAreYouSureText = 1,
		dcdCancelBtn = 2,
		dcdDeleteBtn = 3,
		};
};


/*
**	class SelectCharacterDlg
**
**	asks users to supply a character's name and password,
**	prior to joining the game
*/
class SelectCharacterDlg : public HIDialog
{
private:
	static const OSType Signature = 'SChr';
	static HIViewID		Item( int n ) { return (HIViewID) { Signature, n }; }
	virtual CFStringRef GetNibFileName() const
				{ return CFSTR("Select Character"); }
	
	RestrictEdit		scdCharName;
	PasswordEdit		scdCharPass;
	bool				scdDoDemo;
	
	virtual void	Init();
	virtual bool	DoCommand( const HICommandExtended& cmd );
	void			SaveInfo() const;
	virtual bool	Validate() const;
	
	// relookup passwords whenever the character-name field changes
	virtual bool		FieldDidChangeText( HIViewRef );
	
public:
	// interface routines
	bool			IsDemo() const { return scdDoDemo; }
	
	enum
		{
		scdCharNameEdit	= 1,
		scdCharPassEdit	= 2,
		scdSavePassBox	= 3,
#ifdef ARINDAL
		scdText10		= 5,	// "Haven't signed up yet?"
#endif
		scdDemoBtn		= 4,
		scdPassIcon		= 6,
		};
	enum
		{
		cmd_TryDemo = 'Demo'
		};
};


/*
**	class EnterPasswordDlg
**
**	ask user for a character's password (from DoJoin())
*/
class EnterPasswordDlg : public HIDialog
{
	static const OSType Signature = 'EPwd';
	static HIViewID		Item( int n ) { return (HIViewID) { Signature, n }; }
	virtual CFStringRef GetNibFileName() const
				{ return CFSTR("Enter Password"); }
	
private:
	PasswordEdit		epdCharPass;
	
	// overloaded routines
	virtual void	Init();
	virtual bool	DoCommand( const HICommandExtended& );
	virtual bool	Validate() const;
	
	// interface routines
	void			SaveInfo() const;
	
public:
	enum
		{
		epdCharNameText = 1,
		epdCharPassEdit = 2,
		epdSavePassBox = 3,
		epdPassIcon = 4,
		};
};


/*
**	class CharacterTransferPasswordDlg:
**
**	a bit like EnterPasswordDlg, but used only when
**	confirming a character transfer
*/
class CharacterTransferPasswordDlg : public HIDialog
{
	static const OSType Signature = 'CTPw';		// "Character Transfer"
	static HIViewID		Item( int n ) { return (HIViewID) { Signature, n }; }
	virtual CFStringRef GetNibFileName() const
				{ return CFSTR("Character Transfer"); }
	
private:
	PasswordEdit		bpdCharPass;
	
	virtual void	Init();
	virtual bool	DoCommand( const HICommandExtended& cmd );
	virtual bool	Validate() const;
	void			SaveInfo() const;
	
public:
	enum
		{
		bpdCharNameText = 1,
		bpdCharPassEdit = 2,
		bpdSavePassBox = 3,
		bpdPassIcon = 4,
		};
};


/*
**	class ChangePasswordDlg
**
**	handles the task of requesting a password change
**	for a character (from Char Mgr dlg)
*/
class ChangePasswordDlg : public HIDialog
{
	static const OSType Signature = 'CPwd';
	static HIViewID		Item( int n ) { return (HIViewID) { Signature, n }; }
	virtual CFStringRef GetNibFileName()
				const { return CFSTR("Change Password"); }
	
	PasswordEdit		cpdCharPass;
	PasswordEdit		cpdCharPassAgain;
	
	virtual void	Init();
	virtual bool	DoCommand( const HICommandExtended& );
	virtual bool	Validate() const;
	void			SaveInfo() const;
	
public:
	enum
		{
		cpdCharNameText = 4,
		cpdCharPassEdit	= 1,
		cpdCharPass2Edit = 2,
		cpdSavePassBox	= 3,
		cpdPass1Icon	= 5,
		cpdPass2Icon	= 6,
		};
};


/*
**	class ProxyDlg
**
**	A dialog to allow the user to specify an address for an alternate server.
**	(Yes, it's somewhat misnamed.)  The debug version also provides
**	shortcut access to 'localhost' and the Delta Tao test server.
*/
class ProxyDlg : public HIDialog, public GameTickler
{
	enum
		{
		proxyUseProxyCheck = 1,
		proxyAddressLabel = 7,			// "Server address" or "Proxy address"
		
		proxyAddressEdit = 2,			// these two are...
		proxyAddressDefaultText = 3,	// ... mutually exclusive
		
		proxyUseSpecificPortCheck = 4,
		proxyClientPortEdit = 5,
		};
	
	enum
		{
		cmd_UseProxy	= 'Prox',
		cmd_UsePort		= 'Port',
#ifdef DEBUG_VERSION
		cmd_TestServer	= 'TstA',
		cmd_ThisMachine	= 'LclA',
#endif
		};
	
#ifdef DEBUG_VERSION
	// Interface Builder doesn't like having multiple views with the
	// exact same controlIDs and signatures in the same nib file. So we
	// subtly tweak the signatures in the debug dialog. Sigh.
	
	static const OSType Signature = 'SipA';
#else
	static const OSType Signature = 'SIpA';
#endif
	
	static HIViewID Item( int n ) { return (HIViewID) { Signature, n }; }
	
	virtual CFStringRef	GetNibFileName() const
				{ return CFSTR("Set IP Address"); }
	virtual CFStringRef	GetNibWindowName() const;
	virtual CFStringRef	GetPositionPrefKey() const
				{ return CFSTR("ServerAddressDlgPosition"); }
	
	virtual void	Init();
	virtual bool	DoCommand( const HICommandExtended& cmd );
	
	void			SaveInfo() const;
	void			SetupProxy() const;
};


/*
**	class NamePolicyDlg
**
**	trivial dialog showing hints for choosing good character names
*/
class NamePolicyDlg : public HIDialog
{
	static const OSType Signature = 'Plcy';
	static HIViewID Item( int n ) { return (HIViewID) { Signature, n }; }
	virtual CFStringRef GetNibFileName() const { return CFSTR("Name Advice"); }
	
	enum { npdOKBtn = 1 };
	
	virtual void		Init();
};

#pragma mark -


/*
**	Internal Routines
*/
static DTSError	ConnectCommInfo( char * challenge );
static void		ExtractJoinName( const char * source, char * dest );
static char *	EncodePassword( char * dst, const char * newpw, const char * oldpw );
static DTSError	SendWaitReplyLoop( char * challenge, Message * msg, size_t sz,
					HIViewRef spinVu = nullptr );
static bool		CheckPasswordQuality( const char * password );
static bool		CheckCharNameQuality( const char * name );

#pragma mark -


/*
**	RestrictEdit::Init()
**
**	set up the restricted-edit thingy
*/
void
RestrictEdit::Init( HIViewRef vu, CFStringRef validChars,
					  CFIndex maxLen, const char * iniText,
					  HIViewRef icon /* = nullptr */ )
{
	// stash this stuff
	reControl = vu;
	reMaxLen = maxLen;
	reWarningIcon = icon;
	
	// load the initial text
	SetText( iniText );
	
	// initialize the set of invalid characters, by inverting the 'validChars' set
	if ( CFCharacterSetRef validSet = CFCharacterSetCreateWithCharactersInString(
									kCFAllocatorDefault, validChars ) )
		{
		reInvalidChars = CFCharacterSetCreateInvertedSet( kCFAllocatorDefault,
							validSet );
		CFRelease( validSet );
		}
	
	// set up to notice text-change events
	const EventTypeSpec tEvts[] = 
		{
			{ kEventClassTextField, kEventTextShouldChangeInRange },
			{ kEventClassTextField, kEventTextDidChange }	// MUST be last!
		};
	int nEvts = GetEventTypeCount( tEvts );
	
	// don't need to worry about kEventTextDidChange
	// if we have no associated warning icon
	if ( not icon )
		--nEvts;
	
	__Verify_noErr( InstallControlEventHandler( vu,
						RestrictEdit::HandleTextEvent,
						nEvts, tEvts, this, nullptr ) );
}


/*
**	RestrictEdit::SetText()
**
**	set the contents of the edit-text field
*/
void
RestrictEdit::SetText( const char * text ) const
{
	if ( CFStringRef cs = CreateCFString( text ) )
		{
		__Verify_noErr( HIViewSetText( reControl, cs ) );
		CFRelease( cs );
		
		DidChangeText();
		}
}


/*
**	RestrictEdit::GetText()
**
**	retrieve the contents of the edit field
**	NB: if a non-encodable character somehow sneaks into the field,
**	this function may fail without warning
*/
void
RestrictEdit::GetText( size_t dstLen, char * dst,
				CFStringEncoding enc /* = macRoman */ ) const
{
	if ( CFStringRef cs = HIViewCopyText( reControl ) )
		{
		CFStringGetCString( cs, dst, dstLen, enc );
		CFRelease( cs );
		}
}


/*
**	RestrictEdit::SelectText()
**
**	modify the selection within the field
**	unlike other similarly-named methods, this one doesn't shift focus to
**	the input field (but maybe it ought to?)
*/
void
RestrictEdit::SelectText( int start /* = 0 */, int end /* = 0x7FFF */ ) const
{
	ControlEditTextSelectionRec sel = { start, end };
	__Verify_noErr( SetControlData( reControl,
						kControlEditTextPart, kControlEditTextSelectionTag,
						sizeof sel, &sel ) );
}


/*
**	RestrictEdit::HandleTextEvent()		[static]
**
**	revector text-field events to our virtual method
*/
OSStatus
RestrictEdit::HandleTextEvent( EventHandlerCallRef call, EventRef event, void * ud )
{
	OSStatus result = eventNotHandledErr;
	
	if ( RestrictEdit * re = static_cast<RestrictEdit *>( ud ) )
		{
		const CarbonEvent evt( event, call );
		
		if ( kEventClassTextField == evt.Class() )
			{
			switch ( evt.Kind() )
				{
				case kEventTextShouldChangeInRange:
					result = re->ShouldChangeText( re->reControl, evt );
					break;
				
				case kEventTextDidChange:
					re->DidChangeText();
					break;
				}
			}
		}
	
	return result;
}


/*
**	RestrictEdit::CreateCleanCopy()
**
**	return a copy of 'candidate', with all invalid characters surgically removed
*/
CFStringRef
RestrictEdit::CreateCleanCopy( CFStringRef candidate ) const
{
	// fetch the characters of the text to be purified
	// use a buffer on the stack unless the candidate text is absurdly long
	enum { buffSz = 256 };
	UniChar buff[ buffSz ];
	UniChar * bp = buff;
	
	// may need to allocate a larger buffer
	CFIndex len = CFStringGetLength( candidate );
	if ( len > buffSz )
		bp = NEW_TAG( __PRETTY_FUNCTION__ ) UniChar[ len ];
	
	// now that we surely have room, get the characters into *bp
	CFStringGetCharacters( candidate, CFRangeMake( 0, len ), bp );
	
	// trim out bad characters, one by one
	UniChar * src = bp;
	UniChar * dst = bp;
	CFIndex cleanLen = 0;
	for ( CFIndex ii = 0; ii < len; ++ii, ++src )
		{
		// is this one -not- invalid? In other words, is it OK to keep?
		if ( not CFCharacterSetIsCharacterMember( reInvalidChars, *src ) )
			{
			// NB: within this loop, src will always be >= dst
			if ( dst != src )
				*dst++ = *src;
			
			++cleanLen;
			}
		}
	
	// package up just the good ones
	CFStringRef result = CFStringCreateWithCharacters( kCFAllocatorDefault,
							bp, cleanLen );
	
	// deallocate oversize buffer, if we had one
	if ( bp != buff )
		delete[] bp;
	
	return result;
}


/*
**	RestrictEdit::ShouldChangeText()
**
**	called when the user attempts to modify the contents of the text field.
**	We scan the proposed text and remove any non-valid characters,
**	before allowing the toolbox to proceed with the change.
**	Also attempt to enforce the length limitation.
*/
OSStatus
RestrictEdit::ShouldChangeText( HIViewRef /* vu */, const CarbonEvent& evt ) const
{
	// paranoia: if no valid chars were provided to Init(),
	// then we can't accept -any- input
	if ( not reInvalidChars )
		return noErr;	// meaning "don't insert anything"
	
	OSStatus result = eventNotHandledErr;
	CFStringRef newText = nullptr;	// user's proposed input
	
	// this is a "Get", not a "Copy", so don't release 'newText'
	OSStatus err = evt.GetParameter( kEventParamCandidateText, typeCFStringRef,
						sizeof newText, &newText );
	if ( noErr == err )
		{
		// do we have new text to add/insert?
		CFIndex ntLen = 0;
		if ( newText && (ntLen = CFStringGetLength( newText )) > 0 )
			{
			CFStringRef cleaned = newText;
			CFRetain( cleaned );
			
			// is it too long?
			if ( ntLen > reMaxLen )
				{
				if ( CFStringRef trimmed = CFStringCreateWithSubstring(
											kCFAllocatorDefault, cleaned,
											CFRangeMake( 0, reMaxLen ) ) )
					{
					CFRelease( cleaned );
					cleaned = trimmed;
					}
				}
			
			// does it contain any "bad" characters?
			CFRange foundRange;
			if ( CFStringFindCharacterFromSet( cleaned, reInvalidChars,
					CFRangeMake( 0, ntLen ), kNilOptions, &foundRange ) )
				{
				// uh oh; must filter out the bad ones
				CFStringRef tmp = CreateCleanCopy( cleaned );
				CFRelease( cleaned );
				cleaned = tmp;
				
				// notify user that we dislike what he typed
				Beep();
				}
			
			if ( cleaned )
				{
				if ( CFStringGetLength( cleaned ) > 0 )
					{
					evt.SetParameter( kEventParamReplacementText,
							typeCFStringRef, sizeof cleaned, &cleaned );
					}
				// otherwise, -not- supplying a replacement text is meant to be a
				// sufficient hint to the OS to keep it from inserting anything
				
				// will cause the edit-field to insert 'cleaned' instead of 'newText'
				result = noErr;
				
				CFRelease( cleaned );
				}
			}
		}
	
	return result;
}


/*
**	RestrictEdit::DidChangeText()
**
**	notification that the contents of the input field have changed.
**	if we have an associated warning icon, check if we should hide or show it.
*/
void
RestrictEdit::DidChangeText() const
{
	if ( reWarningIcon )
		{
		bool textOK = Acceptable();
		__Verify_noErr( HIViewSetVisible( reWarningIcon, not textOK ) );
		}
}


/*
**	PasswordEdit::Init()
**
**	init a password-input field, with appropriate size and charset limits
*/
void
PasswordEdit::Init( HIViewRef item, HIViewRef icon, const char * initialText )
{
	RestrictEdit::Init( item, CFSTR(VALID_PASSWORD_CHARS),
						kMaxPassLen, initialText, icon );
}
	

/*
**	PasswordEdit::Acceptable()
**
**	are the current contents of this edit-field OK as a CL password?
*/
bool
PasswordEdit::Acceptable() const
{
	char buff[ kMaxPassLen ];
	GetText( sizeof buff, buff );
	
#if 0	// should we permit an empty field to qualify as "acceptable"?
	if ( '\0' == buff[0] )
		return true;
#endif
	
	return CheckPasswordQuality( buff );
}


/*
**	NameEdit::Init()
**
**	prepare an edit-field, specialized for CL character names
*/
void
NameEdit::Init( HIViewRef item, HIViewRef icon, const char * initialText )
{
	RestrictEdit::Init( item, CFSTR(VALID_NAME_CHARS),
						kMaxJoinLen, initialText, icon );
}


/*
**	NameEdit::Acceptable()
**
**	are the current contents of this edit-field OK as a character name?
*/
bool
NameEdit::Acceptable() const
{
	char buff[ kMaxNameLen ];
	buff[ 0 ] = '\0';
	GetText( sizeof buff, buff );
	
#if 0  // do empty fields qualify as "acceptable"?
	if ( '\0' == buff[ 0 ] )
		return true;
#endif
	
	return CheckCharNameQuality( buff );
};

#pragma mark -


/*
**	DoJoinGame()
**	initiate the process of joining the game, from the Character Manager
**	or Select Char dialogs
*/
static void
DoJoinGame()
{
#if 1
	// post a simulated "mJoinGame" menu event, in a deferred way
	// so that the dialog can finish unwinding before
	// we plunge into that can o' worms.
	
	HICommandExtended cmd;
	cmd.attributes = kHICommandFromMenu;
	cmd.commandID = cmd_JoinGame;
	cmd.source.menu.menuRef = ::GetMenuHandle( rFileMenuID );
	cmd.source.menu.menuItemIndex = LoWord( mJoinGame );
	
	CarbonEvent joinEvt( kEventClassCommand, kEventCommandProcess );
	joinEvt.SetParameter( kEventParamDirectObject,
		typeHICommand, sizeof cmd, &cmd );
	const UInt32 mCtxt = kMenuContextMenuBar | kMenuContextPullDown;
	joinEvt.SetParameter( kEventParamMenuContext,
		typeUInt32, sizeof mCtxt, &mCtxt );
	
	joinEvt.Post( GetApplicationEventTarget(), GetMainEventQueue() );
	
#else  // ! 1
	
	// just jump right in
	gDTSMenu->DoCmd( mJoinGame );
	
#endif  // 1
}


/*
**	CharacterManager()
**
**	jump into the character manager
*/
DTSError
CharacterManager()
{
	CharacterManagerDlg managerDlg;
	char challenge[ kChallengeLen ];
	LogOn& lo = managerDlg.GetMessage()->msgLogOn;
	
	lo.logData[0] = '\0';
	
	bool doDemo = false;
	DTSError result = noErr;
	// if ( noErr == result )
		{
		SelectAccountDlg selectDlg;
		selectDlg.Run();
		
		UInt32 reason = selectDlg.Dismissal();
		if ( kHICommandCancel == reason )
			result = kResultCancel;
		else
		if ( SelectAccountDlg::cmd_TryDemo == reason )
			doDemo = true;
		}
	
	if ( noErr == result )
		result = ConnectCommInfo( challenge );
	if ( noErr == result )
		{
		SetupLogonRequest( &lo, kMsgCharList );
		char * dst = lo.logData;
		if ( doDemo )
			{
			dst = StringCopyDst(   dst, "demo"  );
			dst = AnswerChallenge( dst, "demo", challenge );
			}
		else
			{
			dst = StringCopyDst(   dst, gPrefsData.pdAccount  );
			dst = AnswerChallenge( dst, gPrefsData.pdAcctPass, challenge );
			}
		
		size_t sz = dst - lo.logData;
		SimpleEncrypt( lo.logData, sz );
		sz = dst - reinterpret_cast<char *>( &lo );
		result = SendWaitReplyLoop( challenge, managerDlg.GetMessage(), sz );
		}
	if ( noErr == result )
		result = BigToNativeEndian( lo.logResult );
	
	ExitComm();
	
	if ( noErr == result )
		{
		SimpleEncrypt( lo.logData, sizeof lo.logData );
		managerDlg.SetIsDemo( doDemo );
		managerDlg.Run();
		
		UInt32 reason = managerDlg.Dismissal();
		if ( kHICommandCancel == reason )
			result = kResultCancel;
		else
		if ( managerDlg.cmd_PlayChar == reason )
			DoJoinGame();
		}
	
	if ( noErr > result
	&&   userCanceledErr != result )
		{
		if ( '\0' == lo.logData[0] )
			{
			// "Sorry, there was a failure in the Character Manager with id %d."
			snprintf( lo.logData, sizeof lo.logData,
				_(TXTCL_CHAR_MANAGER_FAILURE),
				int( result ) );
			}
		
		GenericError( "%s", lo.logData );
		}
	
	return result;
}


/*
**	SelectCharacter()
**
**	select an existing character
*/
DTSError
SelectCharacter()
{
	bool doDemo = false;
	
	DTSError result = noErr;
	// if ( noErr == result )
		{
		SelectCharacterDlg dlg;
		dlg.Run();
		
		UInt32 reason = dlg.Dismissal();
		if ( SelectCharacterDlg::cmd_TryDemo == reason
		&&   dlg.IsDemo() )
			{
			doDemo = true;
			}
		else
		if ( kHICommandOK != reason )
			result = kResultCancel;
		}
	
	// we're done if they OKed or canceled
	if ( result != noErr ||	not doDemo )
		return result;
	
	// we're not done if they want the demo
	char challenge[ kChallengeLen ];
	CharacterManagerDlg managerDlg;
	LogOn& lo = managerDlg.GetMessage()->msgLogOn;
	lo.logData[ 0 ] = '\0';
	
	result = ConnectCommInfo( challenge );
	if ( noErr == result )
		{
		SetupLogonRequest( &lo, kMsgCharList );
		char * dst = lo.logData;
		dst = StringCopyDst(   dst, "demo"  );
		dst = AnswerChallenge( dst, "demo", challenge );
		
		size_t sz = dst - lo.logData;
		SimpleEncrypt( lo.logData, sz );
		
		sz = dst - reinterpret_cast<char *>( &lo );
		result = SendWaitReplyLoop( challenge, managerDlg.GetMessage(), sz );
		}
	if ( noErr == result )
		result = BigToNativeEndian( lo.logResult );
	
	ExitComm();
	
	if ( noErr == result )
		{
		SimpleEncrypt( lo.logData, sizeof lo.logData );
		managerDlg.SetIsDemo( doDemo );
		managerDlg.Run();
		
		UInt32 reason = managerDlg.Dismissal();
		if ( managerDlg.cmd_PlayChar == reason )
			{
			DoJoinGame();
			result = kResultCancel;	// to prevent double-joins
			}
		else
		if ( kHICommandCancel == reason )
			result = kResultCancel;
		}
	
	if ( noErr > result
	&&   userCanceledErr != result )
		{
		if ( 0 == lo.logData[0] )
			{
			snprintf( lo.logData, sizeof lo.logData,
				_(TXTCL_CHAR_MANAGER_FAILURE),
				result );
			}
		
		GenericError( "%s", lo.logData );
		}
	
	return result;
}


/*
**	EnterPassword()
**
**	ask user for the password of the current character (from DoJoin())
*/
DTSError
EnterPassword()
{
	EnterPasswordDlg dlg;
	dlg.Run();
	
	if ( kHICommandOK == dlg.Dismissal() )
		return noErr;
	
	return kResultCancel;
}


/*
**	GetHostAddr()
**
**	ask user for an alternate IP or DNS address for the game server
*/
DTSError
GetHostAddr()
{
	ProxyDlg dlg;
	dlg.Run();
	
	if ( kHICommandOK == dlg.Dismissal() )
		return noErr;
		
	return kResultCancel;
}

#pragma mark -


/*
**	SelectAccountDlg::Init()
*/
void
SelectAccountDlg::Init()
{
	KeychainAccess::Get( kPassType_Account, gPrefsData.pdAccount,
					  gPrefsData.pdAcctPass, sizeof gPrefsData.pdAcctPass );
	
//	bool bSavePass = gPrefsData.pdSavePassword;
	bool bSavePass = Prefs::GetBool( pref_SaveAcctPass, false );
	if ( not bSavePass )
		gPrefsData.pdAcctPass[0] = '\0';
	
	sadAcctName.Init( GetItem(Item(sadAcctNameEdit) ), CFSTR(VALID_NAME_CHARS),
		sizeof gPrefsData.pdAccount, gPrefsData.pdAccount );
	HIViewRef icon = GetItem( Item( sadPassIcon ) );
	sadAcctPass.Init( GetItem( Item(sadAcctPassEdit) ),
		icon, gPrefsData.pdAcctPass );
	
	SetFieldLengthLimit( Item( sadAcctNameEdit ), kMaxNameLen );
	SetFieldLengthLimit( Item( sadAcctPassEdit ), kMaxPassLen );
	
	SetValue( Item( sadSavePassBox ), bSavePass );
	
	sadAcctName.SelectText();
	
	// when the account name changes, re-lookup a saved password
	// NOTE: this probably won't work if the account-name field ever gets
	// an "acceptability" icon like the password field has; in that case
	// the RestrictEdit field will have also subscribed to this same
	// text-did-change notification event. We would need a way to arrange for
	// both notification callbacks to get triggered.
	SetFieldNotifications( Item( sadAcctNameEdit ), kTFNotifyDidChange );

#ifdef ARINDAL
	// we don't need the demo button, as Arindal has a different demo system
	SetVisible( Item( sadDemoText ), false );
	SetVisible( Item( sadDemoBtn  ), false );
#endif
}


/*
**	SelectAccountDlg::DoCommand()
**
**	handle button clicks
*/
bool
SelectAccountDlg::DoCommand( const HICommandExtended& cmd )
{
	bool result = true;		// assume we'll handle it
	
	switch ( cmd.commandID )
		{
		case kHICommandOK:
			if ( Validate() )
				{
				SaveInfo();
				Dismiss( cmd.commandID );
				}
			break;
		
		case cmd_TryDemo:
			Dismiss( cmd.commandID );
			break;
		
		default:
			result = false;
		}
	
	return result;
}


/*
**	SelectAccountDlg::FieldDidChangeText()
**
**	if they change the account name, re-lookup its pwd from keychain
*/
bool
SelectAccountDlg::FieldDidChangeText( HIViewRef vu )
{
	bool result = false;
	
	HIViewRef nameView = GetItem( Item(sadAcctNameEdit) );
	if ( vu == nameView )
		{
		char name[ kMaxNameLen ], pass[ 256 ];
		sadAcctName.GetText( sizeof name, name );
		
		if ( noErr == KeychainAccess::Get( kPassType_Account,
						name, pass, sizeof pass )
		&&   pass[ 0 ] )
			{
			sadAcctPass.SetText( pass );
			}
		
		result = true;
		}
	
	return result;
}


/*
**	SelectAccountDlg::Validate()
*/
bool
SelectAccountDlg::Validate() const
{
	char acct[ kMaxNameLen ];
	char pwd [ kMaxPassLen ];
	
	// paranoia
	acct[0] = pwd[0] = '\0';
	
	sadAcctName.GetText( sizeof acct, acct );
	sadAcctPass.GetText( sizeof pwd, pwd );
	
	int badField = 0;
	if ( '\0' == acct[0] )
		{
		badField = sadAcctNameEdit;
		sadAcctName.SelectText();
		}
	else
	if ( '\0' == pwd[0] )
		{
		badField = sadAcctPassEdit;
		sadAcctPass.SelectText();
		}
	
	if ( badField )
		{
		GenericError( _(TXTCL_MUST_FILL_FIELD) );
		
		__Verify_noErr( HIViewSetFocus( GetItem( Item(badField) ),
							kControlEditTextPart, 0 ) );
		return false;
		}
	
	return true;
}


/*
**	SelectAccountDlg::SaveInfo()
*/
void
SelectAccountDlg::SaveInfo() const
{
	// update the prefs
	sadAcctName.GetText( sizeof gPrefsData.pdAccount, gPrefsData.pdAccount );
	sadAcctPass.GetText( sizeof gPrefsData.pdAcctPass, gPrefsData.pdAcctPass );
	
	bool bSavePass = GetValue( Item( sadSavePassBox ) );
//	gPrefsData.pdSavePassword = bSavePass;
	Prefs::Set( pref_SaveAcctPass, bSavePass );
	
	// save (or else delete) the password from the Keychain
	KeychainAccess::Update( kPassType_Account,
		gPrefsData.pdAccount, bSavePass ? gPrefsData.pdAcctPass : "" );
}

#pragma mark -


/*
**	CharacterList::Init()
**
**	prepare the character-list browser
*/
OSStatus
CharacterList::Init( CharacterManagerDlg * dlg, HIViewRef vu )
{
	mParent = dlg;
	OSStatus err = InitFromNib( vu );
	
	if ( noErr == err )
		{
		ChangeAttributes( kDBAttrAlternateColors | kDBAttrAutoHideBars, 0 );
		
		// make the DB autosize its columns when its own size changes
		const EventTypeSpec evts =
			{ kEventClassControl, kEventControlBoundsChanged };
		__Verify_noErr( InstallControlEventHandler( vu,
							DoEvent, 1, &evts, this, nullptr ) );
		}
	
	return err;
}


/*
**	CharacterList::PopulateList()
**
**	tell the browser how many entries it should have
*/
OSStatus
CharacterList::PopulateList()
{
	CFIndex nItems = CountCharacters();
	OSStatus err = AddDBItems( kDataBrowserNoItem,
					nItems, nullptr, prop_CharName );
	return err;
}


/*
**	CharacterList::ParseLogMessage()
**
**	parse the character names out of the kMsgCharList LogOn message,
**	into the 'mChars' array.  Also returns the index into that array
**	of our own character, so we can select it; as well as
**	the number of character slots for this account
**	(that's the total # of slots, both used and free).
*/
void
CharacterList::ParseLogMessage( const Message& msg, int& ourIdx, int& maxChars )
{
	// paranoia
	ourIdx = -1;
	maxChars = 0;
	
	// we'll accumulate char names in this array
	CFMutableArrayRef arr = CFArrayCreateMutable( kCFAllocatorDefault,
								0, &kCFTypeArrayCallBacks );
	__Check( arr );
	if ( not arr )
		return;
	
	// remember our own char name, so we can seek it in the list
	char ourJName[ kMaxJoinLen ];
	CompactName( gPrefsData.pdCharName, ourJName, sizeof ourJName );
	
	// skip 1st three fields (status; paid-up date; max chars)
	const char * ptr = msg.msgLogOn.logData + 3 * sizeof(int32_t);
	
	// examine each name
	for ( int idx = 0; *ptr; ++idx )
		{
		// fish out this one's join-name
		char jn1[ kMaxJoinLen ], jn2[ kMaxJoinLen ];
		ExtractJoinName( ptr, jn1 );
		CompactName( jn1, jn2, sizeof jn2 );
		
		// was this us?
		if ( 0 == strcmp( ourJName, jn2 ) )
			ourIdx = idx;
		
		// stash this name in the array
		if ( CFStringRef str = CreateCFString( ptr ) )
			{
			CFArrayAppendValue( arr, str );
			CFRelease( str );
			}
		
		// move to next
		ptr += strlen( ptr ) + 1;
		}
	
	// replace 'mChars' array with [a non-mutable copy of] 'arr'.
	if ( mChars )
		CFRelease( mChars );
	mChars = CFArrayCreateCopy( kCFAllocatorDefault, arr );
	
	CFRelease( arr );
	
	// how many character-slots does this account have?
	maxChars = BigToNativeEndian( ((int32_t *) msg.msgLogOn.logData)[2] );
}


/*
**	CharacterList::CountCharacters()
**
**	how many characters on this account?
*/
int
CharacterList::CountCharacters() const
{
	if ( mChars )
		return CFArrayGetCount( mChars );
	
	return 0;
}


/*
**	CharacterList::GetNthCharName()
**
**	accessor for the char-names in the mChars array
*/
CFStringRef
CharacterList::GetNthCharName( int n ) const
{
	CFStringRef result = nullptr;
	
	if ( mChars )
		{
		CFTypeRef tr = CFArrayGetValueAtIndex( mChars, n );
		if ( tr && CFStringGetTypeID() == CFGetTypeID( tr ) )
			result = static_cast<CFStringRef>( tr );
		}
	
	return result;
}


/*
**	CharacterList::GetItemData()
**
**	provide info for the databrowser, specifically the characters' names
*/
OSStatus
CharacterList::GetItemData(
	DataBrowserItemID item,
	DataBrowserPropertyID prop,
	DataBrowserItemDataRef dr )
{
	OSStatus result = errDataBrowserPropertyNotSupported;
	if ( prop_CharName == prop )
		{
		int idx = item - 1;
		CFStringRef name = GetNthCharName( idx );
		result = SetDataBrowserItemDataText( dr, name );
		__Check_noErr( result );
		}
	return result;
}


/*
**	CharacterList::DoSelectionSetChanged()
**
**	callback when the user modifies the selection in the databrowser.
**	ask the owning dialog to update button states accordingly
*/
void
CharacterList::DoSelectionSetChanged( DataBrowserItemID /* item */,
					DataBrowserItemDataRef /* prop */ )
{
	mParent->UpdateButtons();
}


/*
**	CharacterList::DoDoubleClick()
**
**	react to a double-click in the list of character names
*/
void
CharacterList::DoDoubleClick( DataBrowserItemID /* item */,
							  DataBrowserItemDataRef /* data */ )
{
	mParent->Dismiss( CharacterManagerDlg::cmd_PlayChar );
}


/*
**	CharacterList::DoEvent()		[static]
**
**	forward Carbon Events to the instance method
*/
OSStatus
CharacterList::DoEvent( EventHandlerCallRef call, EventRef event, void * ud )
{
	OSStatus result = eventNotHandledErr;
	CharacterList * clist = static_cast<CharacterList *>( ud );
	if ( clist )
		{
		CarbonEvent evt( event, call );
		result = clist->HandleEvent( evt );
		}
	return result;
}


/*
**	CharacterList::HandleEvent()
**
**	handle carbon events -- specifically, when the DB resizes
*/
OSStatus
CharacterList::HandleEvent( CarbonEvent& evt )
{
	OSStatus result = eventNotHandledErr;
	
	if ( kEventClassControl == evt.Class() )
		{
		switch ( evt.Kind() )
			{
			case kEventControlBoundsChanged:
				{
				UInt32 atts( 0 );
				evt.GetParameter( kEventParamAttributes,
						typeUInt32, sizeof atts, &atts );
				if ( atts & kControlBoundsChangeSizeChanged )
					__Verify_noErr( AutoSizeDataBrowserListViewColumns( *this ) );
				}
			break;
			}
		}
	
	return result;
}

#pragma mark -


/*
**	PackedDateToString()
**
**	given a paid-up date in packed YYYYMMDD format, produce a nicely formatted
**	human readable representation (e.g. "Fri, 7 Feb 2014")
*/
static void
PackedDateToString( int paidup, char * buff, size_t maxlen )
{
	// pessimism
	StringCopySafe( buff, "????", maxlen );
	
	// convert 'paidup' into a CFAbsoluteTime
	CFAbsoluteTime paidUpTime = 0;
	if ( CFCalendarRef cal = CFCalendarCreateWithIdentifier( kCFAllocatorDefault,
								kCFGregorianCalendar ) )
		{
		int year  = (paidup >> 16) & 0x0FFFF;
		int month = (paidup >> 8) & 0x0FF;
		int day   = paidup & 0x0FF;
		
		if ( not CFCalendarComposeAbsoluteTime( cal,
					&paidUpTime, "yMd", year, month, day ) )
			{
			// we're hosed
			CFRelease( cal );
			return;
			}
		CFRelease( cal );
		}
	
	// create a date-formatter the way we like it
	CFDateFormatterRef fmtr = nullptr;
	if ( CFLocaleRef locale = CFLocaleCopyCurrent() )
		{
		fmtr = CFDateFormatterCreate( kCFAllocatorDefault,
				locale,
				kCFDateFormatterNoStyle,	// no styles specified, because...
				kCFDateFormatterNoStyle );	// ... the format _string_ overrides them
		
		if ( fmtr )
			{
			// For the meanings of the format-string codes, see:
			//	http://unicode.org/reports/tr35/tr35-6.html#Date_Format_Patterns
			// Briefly: E is weekday name; the others are self-explanatory
			CFDateFormatterSetFormat( fmtr, CFSTR("E, d MMM y") );
			}
		
		CFRelease( locale );
		}
	
	// get the formatted date as a CFString
	CFStringRef paidUpStr = nullptr;
	if ( fmtr )
		{
		paidUpStr = CFDateFormatterCreateStringWithAbsoluteTime(
						kCFAllocatorDefault, fmtr, paidUpTime );
		CFRelease( fmtr );
		}
	
	// extract the C string
	if ( paidUpStr )
		{
		// try MacRoman first; if that fails, as UTF8
		if ( not CFStringGetCString( paidUpStr,
					buff, maxlen, kCFStringEncodingMacRoman ) )
			{
			CFStringGetCString( paidUpStr, buff, maxlen, kCFStringEncodingUTF8 );
			}
		
		CFRelease( paidUpStr );
		}
}


/*
**	CharacterManagerDlg::Init()
*/
void
CharacterManagerDlg::Init()
{
	// initialize the list
	cmdCharList.Init( this, GetItem( Item(cmdCharacterList) ) );
	__Verify_noErr( SetWindowCancelButton( *this, GetItem(Item( cmdDone ) ) ) );
	
	char buff[256];
	
	// double-check for demo-hood
	StringCopySafe( buff, gPrefsData.pdAccount, sizeof buff );
	StringToUpper( buff );
	
#ifndef DEBUG_VERSION
	// this allows a sort of backdoor so that a GM with the debug client
	// can do maintenance on the demo account, if necessary. I think.
	if ( 0 == strcmp( buff, "DEMO" ) )
		cmdIsDemo = true;
#endif
	
	if ( cmdIsDemo )
		SetText( Item(cmdAccountText), _(TXTCL_CLICK_ON_DEMO_CHARS) );
	else
		{
		// show the account name in the text
		snprintf( buff, sizeof buff,
			_(TXTCL_CHARS_FOR_ACCOUNT), gPrefsData.pdAccount );
		SetText( Item(cmdAccountText), buff );
		}
	
	// extract account information
	const int32_t * logData =
		reinterpret_cast<int32_t *>( &cmdMsg.msgLogOn.logData );
	int status   = BigToNativeEndian( logData[0] );
	int paidup   = BigToNativeEndian( logData[1] );
	int maxchars = BigToNativeEndian( logData[2] );
	
	// bail if there is a problem with the account
	if ( kAcctStatusNormal != status
	&&   kAcctStatusDemoAcct != status )
		{
		SetText( Item(cmdFreeText), _(TXTCL_PROBLEM_WITH_ACCOUNT) );
		return;
		}
	
	// show the maximum number of characters and the paid up date
	char * ptr = buff;
	if ( 1 == maxchars )
		{
#ifdef ARINDAL
		if ( kAcctStatusDemoAcct == status )
			ptr += snprintfx( buff, sizeof buff, _(TXTCL_PAID_ONE_DEMOCHAR) );
		else
#endif
			ptr += snprintfx( buff, sizeof buff, _(TXTCL_PAID_ONE_CHAR) );
		}
	else
		{
#ifdef ARINDAL
		if ( kAcctStatusDemoAcct == status )
			ptr += snprintfx( buff, sizeof buff,
					_(TXTCL_PAID_X_DEMOCHARS), maxchars );
		else
#endif
			ptr += snprintfx( buff, sizeof buff, _(TXTCL_PAID_X_CHARS), maxchars );
		}
	
#if DISPLAY_PAID_UP_DATES || defined( DEBUG_VERSION )
	// format 'paidup' into 'dateCStr' nicely
	char dateCStr[ 256 ];
	PackedDateToString( paidup, dateCStr, sizeof dateCStr );
	
	// fetch today, in packed-date format
	DTSDate date;
	date.Get();
	int today	= ( date.dateYear  << 16 )
				+ ( date.dateMonth <<  8 )
				+ ( date.dateDay   <<  0 );
	
	if ( today < paidup )
		sprintf( ptr, _(TXTCL_ACCOUNT_PAID_UNTIL), dateCStr );
	else
	if ( today == paidup )
		strcpy( ptr, _(TXTCL_ACCOUNT_EXPIRE_TODAY) );
	else
		sprintf( ptr, _(TXTCL_ACCOUNT_EXPIRED_ON), dateCStr );
#endif  // DISPLAY_PAID_UP_DATES
	
	if ( not cmdIsDemo )
		SetText( Item(cmdFreeText), buff );
	else
		SetText( Item(cmdFreeText), _(TXTCL_DEMO_CHAR_DESC) );
	
	InitCharList();
}


/*
**	InitCharList()
*/
void
CharacterManagerDlg::InitCharList()
{
	// clear the list; reset the buttons
	
	// clear prior selection if any (after delete)
	cmdCharList.RemoveAllDBItems();
	
	AbleItem( Item(cmdPlayBtn), false );		// can't play if no selection
	AbleItem( Item(cmdDeleteBtn), false );		// nor delete
	AbleItem( Item(cmdPasswordBtn), false );	// nor password
	__Verify_noErr( SetWindowDefaultButton( *this, GetItem(Item(cmdDone) ) ) );
	
	// put the character names in the list box
	int ourIdx, maxChars;
	cmdCharList.ParseLogMessage( cmdMsg, ourIdx, maxChars );
	cmdCharList.PopulateList();
	
	// if ourIdx is valid, select that one (it's us)
	if ( ourIdx != -1 )
		cmdCharList.SetDBSelectedItem( ourIdx + 1 );
	
	UpdateButtons();
	
	// disable "create" if you're at your limit or we're running the demo
	AbleItem( Item( cmdCreateBtn ),
		cmdCharList.CountCharacters() < maxChars && not cmdIsDemo );
}


/*
**	CharacterManagerDlg::Exit()
*/
void
CharacterManagerDlg::Exit()
{
	cmdCharList.Exit();
}


/*
**	CharacterManagerDlg::DoCommand()
**
**	react to button clicks
*/
bool
CharacterManagerDlg::DoCommand( const HICommandExtended& cmd )
{
	bool result = true;
	switch ( cmd.commandID )
		{
		case cmd_PlayChar:
			SaveInfo();
			Dismiss( cmd.commandID );
			break;
		
		case cmd_CreateChar:
			DoCreate();
			break;
		
		case cmd_DeleteChar:
			DoDelete();
			break;
		
		case cmd_Password:
			DoPassword();
			break;
		
		default:
			result = false;
		}
	
	return result;
}


/*
**	CharacterManagerDlg::UpdateButtons()
**
**	set state of the buttons to correspond to
**	what's selected in the browser (if anything)
*/
void
CharacterManagerDlg::UpdateButtons() const
{
	DataBrowserItemID iid = kDataBrowserNoItem;
	
	// something selected?
	bool bOK = false;
	if ( noErr == cmdCharList.GetDBSelectedItem( &iid ) )
		{
		int idx = iid - 1;
	
		bOK = idx >= 0 && idx < cmdCharList.CountCharacters();
		}
	
	AbleItem( Item( cmdPlayBtn ), bOK );
	
	AbleItem( Item( cmdDeleteBtn ), bOK && not cmdIsDemo );
	AbleItem( Item( cmdPasswordBtn ), bOK && not cmdIsDemo );
	
	__Verify_noErr( SetWindowDefaultButton( *this,
		GetItem( Item( bOK ? cmdPlayBtn : cmdDone ) ) ) );
}


/*
**	CharacterManagerDlg::SaveInfo()
*/
void
CharacterManagerDlg::SaveInfo() const
{
	char buff[ 256 ];
	DTSError result = GetSelectedName( buff, sizeof buff );
	if ( noErr == result )
		{
		char joinname[ kMaxJoinLen ];
		ExtractJoinName( buff, joinname );
		
		StringCopySafe( gPrefsData.pdCharName,
			joinname, sizeof gPrefsData.pdCharName );
		
		if ( cmdIsDemo )
			strcpy( gPrefsData.pdCharPass, "demo" );
		}
}


/*
**	CharacterManagerDlg::DoCreate()
*/
void
CharacterManagerDlg::DoCreate()
{
	DTSError result = noErr;
	// if ( noErr == result )
		{
		CreateCharacterDlg newCharDlg;
		newCharDlg.Run();
		
		if ( kHICommandOK != newCharDlg.Dismissal() )
			result = kResultCancel;
		}
	
	if ( noErr == result )
		{
		const char * charpass = gPrefsData.pdAcctPass;
		for (;;)
			{
			result = DoCreateLoop( charpass );
			
			// kBadBetaPass now means "collect verification password
			// for phase 2 of a character transfer"
			if ( kBadBetaPass != result )
				break;
			result = DoTransferPass();
			if ( result != noErr )
				break;
			charpass = gPrefsData.pdCharPass;
			}
		}
	
	if ( noErr > result
	&&   userCanceledErr != result )
		{
		GenericError( _(TXTCL_FAILED_CREATE_CHAR),
			result,
			cmdMsg.msgLogOn.logData );
		}
}


/*
**	CharacterManagerDlg::DoCreateLoop()
*/
DTSError
CharacterManagerDlg::DoCreateLoop( const char * charpass )
{
	char challenge[ kChallengeLen ];
	LogOn& lo = cmdMsg.msgLogOn;
	
	DTSError result = ConnectCommInfo( challenge );
	if ( noErr == result )
		{
		SetupLogonRequest( &lo, kMsgNewChar );
		char * dst = lo.logData;
		dst = StringCopyDst(   dst, gPrefsData.pdAccount  );
		dst = AnswerChallenge( dst, gPrefsData.pdAcctPass, challenge );
		dst = StringCopyDst(   dst, gPrefsData.pdCharName );
		dst = EncodePassword(  dst, charpass, gPrefsData.pdAcctPass );
		
		size_t sz = dst - lo.logData;
		SimpleEncrypt( lo.logData, sz );
		
		sz = dst - reinterpret_cast<char *>( &lo );
		result = SendWaitReplyLoop( challenge, &cmdMsg, sz, GetSpinnerView() );
		}
	if ( noErr == result )
		result = BigToNativeEndian( lo.logResult );
	
	ExitComm();
	
	if ( noErr == result )
		{
		char buff[ 256 ];
		snprintf( buff, sizeof buff, _(TXTCL_CHAR_CREATED),
			gPrefsData.pdCharName );
		ShowMsgDlg( buff );
		SimpleEncrypt( lo.logData, sizeof lo.logData );
		InitCharList();
		}
	
	return result;
}


/*
**	CharacterManagerDlg::DoTransferPass()
**
**	enter a password for a non-attached character (used to be "beta").
**	used when finalizing a character transfer from one account to another
*/
DTSError
CharacterManagerDlg::DoTransferPass()
{
	CharacterTransferPasswordDlg dlg;
	dlg.Run();
	
	if ( kHICommandOK == dlg.Dismissal() )
		return noErr;
	
	return kResultCancel;
}


/*
**	CharacterManagerDlg::GetSelectedName()
*/
DTSError
CharacterManagerDlg::GetSelectedName( char * dst, size_t dstLen ) const
{
	DTSError result = noErr;

	dst[ 0 ] = '\0';
	
	DataBrowserItemID iid;
	result = cmdCharList.GetDBSelectedItem( &iid );
	if ( noErr == result )
		{
		int idx = iid - 1;
		if ( CFStringRef name = cmdCharList.GetNthCharName( idx ) )
			{
			if ( not CFStringGetCString( name,
					dst, dstLen, kCFStringEncodingMacRoman ) )
				{
				result = paramErr;
				}
			}
		}
	
	return result;
}


/*
**	CharacterManagerDlg::DoDelete()
*/
void
CharacterManagerDlg::DoDelete()
{
	char buff[ 256 ];
	char challenge[ kChallengeLen ];
	LogOn& lo = cmdMsg.msgLogOn;
	
	DTSError result = GetSelectedName( buff, sizeof buff );
	if ( noErr == result )
		{
		ExtractJoinName( buff, gPrefsData.pdCharName );
		
		DeleteCharacterDlg deleteDlg;
		deleteDlg.Run();
		if ( kHICommandOK != deleteDlg.Dismissal() )
			result = kResultCancel;
		}
	if ( noErr == result )
		result = ConnectCommInfo( challenge );
	if ( noErr == result )
		{
		SetupLogonRequest( &lo, kMsgDeleteChar );
		char * dst = lo.logData;
		dst = StringCopyDst(   dst, gPrefsData.pdAccount  );
		dst = AnswerChallenge( dst, gPrefsData.pdAcctPass, challenge );
		dst = StringCopyDst(   dst, gPrefsData.pdCharName );
		
		size_t sz = dst - lo.logData;
		SimpleEncrypt( lo.logData, sz );
		
		sz = dst - reinterpret_cast<char *>( &lo );
		result = SendWaitReplyLoop( challenge, &cmdMsg, sz, GetSpinnerView() );
		}
	if ( noErr == result )
		result = BigToNativeEndian( lo.logResult );
	
	ExitComm();
	
	if ( noErr == result )
		{
		snprintf( buff, sizeof buff, _(TXTCL_CHAR_DELETED), gPrefsData.pdCharName );
		ShowMsgDlg( buff );
		gPrefsData.pdCharName[0] = '\0';
		SimpleEncrypt( lo.logData, sizeof lo.logData );
		InitCharList();
		}
	if ( noErr > result  &&  userCanceledErr != result )
		{
		GenericError( _(TXTCL_FAILED_DELETE_CHAR),
			gPrefsData.pdCharName,
			result,
			lo.logData );
		}
}


/*
**	CharacterManagerDlg::DoPassword()
*/
void
CharacterManagerDlg::DoPassword()
{
	char buff[ 256 ];
	char challenge[ kChallengeLen ];
	LogOn& lo = cmdMsg.msgLogOn;
	
	DTSError result = GetSelectedName( buff, sizeof buff );
	if ( noErr == result )
		{
		ExtractJoinName( buff, gPrefsData.pdCharName );
		
		ChangePasswordDlg passwordDlg;
		passwordDlg.Run();

		if ( kHICommandOK != passwordDlg.Dismissal() )
			result = kResultCancel;
		}
	if ( noErr == result )
		result = ConnectCommInfo( challenge );
	if ( noErr == result )
		{
		SetupLogonRequest( &lo, kMsgNewPassword );
		char * dst = lo.logData;
		dst = StringCopyDst(   dst, gPrefsData.pdAccount  );
		dst = AnswerChallenge( dst, gPrefsData.pdAcctPass, challenge );
		dst = StringCopyDst(   dst, gPrefsData.pdCharName );
		dst = EncodePassword(  dst, gPrefsData.pdCharPass, gPrefsData.pdAcctPass );
		
		size_t sz = dst - lo.logData;
		SimpleEncrypt( lo.logData, sz );
		
		sz = dst - reinterpret_cast<char *>( &lo );
		result = SendWaitReplyLoop( challenge, &cmdMsg, sz, GetSpinnerView() );
		}
	if ( noErr == result )
		result = BigToNativeEndian( lo.logResult );
	
	ExitComm();
	
	if ( noErr == result )
		{
		snprintf( buff, sizeof buff, _(TXTCL_PASS_CHANGED),
			gPrefsData.pdCharName );
		ShowMsgDlg( buff );
		}
	
	if ( noErr > result
	&&   userCanceledErr != result )
		{
		GenericError( _(TXTCL_FAILED_CHANGE_PASS),
			gPrefsData.pdCharName,
			result,
			lo.logData );
		}
}

#pragma mark -


/*
**	CreateCharacterDlg::Init()
*/
void
CreateCharacterDlg::Init()
{
	HIViewRef nameVu = GetItem( Item( ccdCharNameEdit ) );
	HIViewRef icon = GetItem( Item( ccdBadNameIcon ) );
	ccdCharName.Init( nameVu, icon, "" /* gPrefsData.pdCharName */ );
	ccdCharName.SelectText();
}


/*
**	CreateCharacterDlg::DoCommand()
**	react to button clicks
*/
bool
CreateCharacterDlg::DoCommand( const HICommandExtended& cmd )
{
	bool result = true;
	
	switch ( cmd.commandID )
		{
		case kHICommandOK:
			if ( Validate() )
				{
				SaveInfo();
				Dismiss( cmd.commandID );
				}
			break;
		
		case cmd_NamePolicy:
			{
			// run this as a sheet
			NamePolicyDlg * dlg = NEW_TAG( __PRETTY_FUNCTION__ ) NamePolicyDlg;
			dlg->Run( *this );
			}
			break;
		
		default:
			result = false;
		}
	
	return result;
}


/*
**	CreateCharacterDlg::Validate()
*/
bool
CreateCharacterDlg::Validate() const
{
	char name[ kMaxNameLen ];
	name[ 0 ] = '\0';
	
	ccdCharName.GetText( sizeof name, name );
	
	int badField = 0;
	if ( '\0' == name[0] )
		{
		badField = ccdCharNameEdit;
		ccdCharName.SelectText();
		}
	if ( badField )
		{
		GenericError( _(TXTCL_MUST_FILL_FIELD) );
		return false;
		}
	
	return true;
}


/*
**	CreateCharacterDlg::SaveInfo()
*/
void
CreateCharacterDlg::SaveInfo() const
{
	gPrefsData.pdCharName[ 0 ] = '\0';
	ccdCharName.GetText( sizeof gPrefsData.pdCharName, gPrefsData.pdCharName );
}


/*
**	NamePolicyDlg::Init()
**
**	init the name-policy sheet
*/
void
NamePolicyDlg::Init()
{
	// set the OK button as the cancel button too
	if ( HIViewRef okBtn = GetItem( Item( npdOKBtn ) ) )
		{
		__Verify_noErr( SetWindowCancelButton( *this, okBtn ) );
		}
}

#pragma mark -


/*
**	DeleteCharacterDlg::Init()
*/
void
DeleteCharacterDlg::Init()
{
	CFStringRef cs = nullptr;
	
	// "Are you ... character "%s"?"
	if ( CFStringRef fmt = CFCopyLocalizedString(
						CFSTR("ARE_SURE_PERM_DELETE"),
						CFSTR("part of the delete-character confirmation dialog") ) )
		{
		cs = CFStringCreateFormatted( fmt, gPrefsData.pdCharName );
		CFRelease( fmt );
		}
	
	if ( cs )
		{
		SetText( Item( dcdAreYouSureText ), cs );
		CFRelease( cs );
		}
	
	// make "Cancel" the default, for safety
	if ( HIViewRef cancelBtn = GetItem( Item( dcdCancelBtn ) ) )
		{
		__Verify_noErr( SetWindowDefaultButton( *this, cancelBtn ) );
		}
}

#pragma mark -


/*
**	SelectCharacterDlg::Init()
*/
void
SelectCharacterDlg::Init()
{
	scdDoDemo = false;
	
	bool bSavePass = gPrefsData.pdSavePassword;
	if ( not bSavePass )
		gPrefsData.pdCharPass[0] = '\0';
	else
		{
		KeychainAccess::Get( kPassType_Character, gPrefsData.pdCharName,
			gPrefsData.pdCharPass, sizeof gPrefsData.pdCharPass );
		}
	
	HIViewRef vu = GetItem( Item( scdCharNameEdit ) );
	scdCharName.Init( vu, CFSTR(VALID_NAME_CHARS),
		sizeof gPrefsData.pdCharName, gPrefsData.pdCharName );
	
	vu = GetItem( Item( scdCharPassEdit ) );
	HIViewRef icon = GetItem( Item( scdPassIcon ) );
	scdCharPass.Init( vu, icon, gPrefsData.pdCharPass );
	
	// to relookup passwords whenever the character-name field changes
	SetFieldNotifications( Item( scdCharNameEdit ), kTFNotifyDidChange );
	
	SetValue( Item( scdSavePassBox ), bSavePass );
	scdCharName.SelectText();
	
#ifdef ARINDAL
	// we don't need the demo button as Arindal has a different demo system
	HideItem( scdText10 );      // the "haven't signed up?" text
	HideItem( scdDemoBtn );     // the "Try Demo!" button
#endif
}


/*
**	SelectCharacterDlg::DoCommand()
**
**	react to the buttons
*/
bool
SelectCharacterDlg::DoCommand( const HICommandExtended& cmd )
{
	bool result = true;
	
	switch ( cmd.commandID )
		{
		case kHICommandOK:
			if ( Validate() )
				{
				SaveInfo();
				Dismiss( cmd.commandID );
				}
			break;
		
		case cmd_TryDemo:
			scdDoDemo = true;
			Dismiss( cmd.commandID );
			break;
		
		default:
			result = false;
		}
	
	return result;
}


/*
**	SelectCharacterDlg::FieldDidChangeText()
**
**	notice when the char-name field changes
**	so we can re-lookup the associated password
*/
bool
SelectCharacterDlg::FieldDidChangeText( HIViewRef vu )
{
	bool result = false;
	HIViewRef nameVu = GetItem( Item( scdCharNameEdit ) );
	if ( vu == nameVu )
		{
		char name[ kMaxNameLen ], pass[ 256 ];
		scdCharName.GetText( sizeof name, name );
		
		if ( noErr == KeychainAccess::Get( kPassType_Character,
						name, pass, sizeof pass )
		&&   pass[ 0 ] )
			{
			scdCharPass.SetText( pass );
			}
		
		result = true;
		}
	
	return result;
}


/*
**	SelectCharacterDlg::Validate()
*/
bool
SelectCharacterDlg::Validate() const
{
	char name[ kMaxNameLen ];
	char pwd [ kMaxPassLen ];
	name[0] = pwd[0] = '\0';
	
	scdCharName.GetText( sizeof name, name );
	scdCharPass.GetText( sizeof pwd,  pwd  );
	
	int badField = 0;
	if ( '\0' == name[0] )
		{
		badField = scdCharNameEdit;
		scdCharName.SelectText();
		}
	else
	if ( '\0' == pwd[0] )
		{
		badField = scdCharPassEdit;
		scdCharPass.SelectText();
		}
	if ( badField )
		{
		GenericError( _(TXTCL_MUST_FILL_FIELD) );
		return false;
		}
	
	return true;
}


/*
**	SelectCharacterDlg::SaveInfo()
*/
void
SelectCharacterDlg::SaveInfo() const
{
	char name[ kMaxNameLen ];
	name[0] = '\0';
	scdCharName.GetText( sizeof name, name );
	
	// trim leading spaces from name
	char * pBuffer = name;
	while ( ' ' == *pBuffer )
		++pBuffer;
	
	StringCopySafe( gPrefsData.pdCharName, pBuffer, sizeof gPrefsData.pdCharName );
	
	// trim trailing spaces
	pBuffer = gPrefsData.pdCharName + strlen( gPrefsData.pdCharName ) - 1;
	while ( ' ' == *pBuffer )
		--pBuffer;
	pBuffer[1] = '\0';
	
	gPrefsData.pdCharPass[ 0 ] = '\0';
	scdCharPass.GetText( sizeof gPrefsData.pdCharPass, gPrefsData.pdCharPass );
	
	gPrefsData.pdSavePassword = GetValue( Item( scdSavePassBox ) );
	
	// save pwd in keychain (or delete it)
	KeychainAccess::Update( kPassType_Character, gPrefsData.pdCharName,
		gPrefsData.pdSavePassword ? gPrefsData.pdCharPass : "" );
}

#pragma mark -


/*
**	EnterPasswordDlg::Init()
*/
void
EnterPasswordDlg::Init()
{
	KeychainAccess::Get( kPassType_Character, gPrefsData.pdCharName,
						gPrefsData.pdCharPass, sizeof gPrefsData.pdCharPass );
	
	bool bSavePass = gPrefsData.pdSavePassword;
	if ( not bSavePass )
		gPrefsData.pdCharPass[0] = '\0';
	
	SetText( Item(epdCharNameText), gPrefsData.pdCharName );
	
	HIViewRef passVu = GetItem( Item(epdCharPassEdit) ); 
	HIViewRef icon = GetItem( Item( epdPassIcon ) );
	epdCharPass.Init( passVu, icon, gPrefsData.pdCharPass );
	
	SetValue( Item(epdSavePassBox), bSavePass );
	
	epdCharPass.SelectText();
}


/*
**	EnterPasswordDlg::DoCommand()
**	react to button clicks
*/
bool
EnterPasswordDlg::DoCommand( const HICommandExtended& cmd )
{
	if ( kHICommandOK == cmd.commandID )
		{
		if ( Validate() )
			{
			SaveInfo();
			Dismiss( cmd.commandID );
			}
		return true;
		}
		
	return false;	// let superclass finish handling OK, cancel, etc.
}


/*
**	EnterPasswordDlg::Validate()
*/
bool
EnterPasswordDlg::Validate() const
{
	char pwd[ kMaxPassLen ];
	pwd[0] = '\0';
	epdCharPass.GetText( sizeof pwd, pwd );
	
	int badField = 0;
	if ( '\0' == pwd[0] )
		{
		badField = epdCharPassEdit;
		epdCharPass.SelectText();
		}
	if ( badField )
		{
		GenericError( _(TXTCL_MUST_FILL_FIELD) );
		return false;
		}
	
	return true;
}


/*
**	EnterPasswordDlg::SaveInfo()
*/
void
EnterPasswordDlg::SaveInfo() const
{
	gPrefsData.pdCharPass[ 0 ] = '\0';
	epdCharPass.GetText( sizeof gPrefsData.pdCharPass, gPrefsData.pdCharPass );
	
	gPrefsData.pdSavePassword = GetValue( Item( epdSavePassBox) );
}

#pragma mark -


/*
**	CharacterTransferPasswordDlg::Init()
*/
void
CharacterTransferPasswordDlg::Init()
{
	bool bSavePass = gPrefsData.pdSavePassword;
	if ( not bSavePass )
		gPrefsData.pdCharPass[ 0 ] = '\0';
	
	SetText( Item( bpdCharNameText ), gPrefsData.pdCharName );
	
	HIViewRef pwdVu = GetItem( Item( bpdCharPassEdit ) );
	HIViewRef icon = GetItem( Item( bpdPassIcon ) );
	bpdCharPass.Init( pwdVu, icon, gPrefsData.pdCharPass );
	
	SetValue( Item(bpdSavePassBox), bSavePass );
	
	bpdCharPass.SelectText();
}


/*
**	CharacterTransferPasswordDlg::DoCommand()
*/
bool
CharacterTransferPasswordDlg::DoCommand( const HICommandExtended& cmd )
{
	if ( kHICommandOK == cmd.commandID )
		{
		if ( Validate() )
			{
			SaveInfo();
			Dismiss( cmd.commandID );
			}
		return true;
		}
	
	return false;
}


/*
**	CharacterTransferPasswordDlg::Validate()
*/
bool
CharacterTransferPasswordDlg::Validate() const
{
	char pwd[ kMaxPassLen ];
	pwd[ 0 ] = '\0';
	bpdCharPass.GetText( sizeof pwd, pwd );
	
	int badField = 0;
	if ( '\0' == pwd[0] )
		{
		badField = bpdCharPassEdit;
		bpdCharPass.SelectText();
		}
	if ( badField )
		{
		GenericError( _(TXTCL_MUST_FILL_FIELD) );
		return false;
		}
	
	return true;
}


/*
**	CharacterTransferPasswordDlg::SaveInfo()
*/
void
CharacterTransferPasswordDlg::SaveInfo() const
{
	gPrefsData.pdCharPass[ 0 ] = '\0';
	bpdCharPass.GetText( sizeof gPrefsData.pdCharPass, gPrefsData.pdCharPass );
	
	gPrefsData.pdSavePassword = GetValue( Item( bpdSavePassBox ) );
}

#pragma mark -


/*
**	ChangePasswordDlg::Init()
*/
void
ChangePasswordDlg::Init()
{
	bool bSavePass = gPrefsData.pdSavePassword;
	if ( not bSavePass )
		gPrefsData.pdCharPass[ 0 ] = '\0';
	
	CFStringRef label = nullptr;
	// "Please enter a new password for the character \"%s\":"
	if ( CFStringRef fmt = CFCopyLocalizedString( CFSTR("PASSWORD_DLG_PROMPT"),
							CFSTR("part of the change-password dialog") ) )
		{
		label = CFStringCreateFormatted( fmt, gPrefsData.pdCharName );
		CFRelease( fmt );
		}
	if ( label )
		{
		SetText( Item( cpdCharNameText ), label );
		CFRelease( label );
		}
	
	HIViewRef vu = GetItem( Item( cpdCharPassEdit ) );
	HIViewRef icon = GetItem( Item( cpdPass1Icon ) );
	cpdCharPass.Init( vu, icon, "" );
	
	vu = GetItem( Item( cpdCharPass2Edit ) );
	icon = GetItem( Item( cpdPass2Icon ) );
	cpdCharPassAgain.Init( vu, icon, "" );
	
	SetValue( Item(cpdSavePassBox), bSavePass );
	
	cpdCharPass.SelectText();
}


/*
**	ChangePasswordDlg::DoCommand()
*/
bool
ChangePasswordDlg::DoCommand( const HICommandExtended& cmd )
{
	if ( kHICommandOK == cmd.commandID )
		{
		if ( Validate() )
			{
			SaveInfo();
			Dismiss( cmd.commandID );
			}
		return true;
		}
	
	return false;
}


/*
**	ChangePasswordDlg::SaveInfo()
*/
void
ChangePasswordDlg::SaveInfo() const
{
	gPrefsData.pdCharPass[ 0 ] = '\0';
	cpdCharPass.GetText( sizeof gPrefsData.pdCharPass, gPrefsData.pdCharPass );
	
	gPrefsData.pdSavePassword = bool( GetValue( Item( cpdSavePassBox ) ) );
}


/*
**	ChangePasswordDlg::Validate()
**
**	user clicked OK. But is it?
*/
bool
ChangePasswordDlg::Validate() const
{
	// both fields filled in?
	char pwd [ kMaxPassLen ];
	char pwd2[ kMaxPassLen ];
	pwd[0] = pwd2[0] = '\0';
	
	cpdCharPass.GetText( sizeof pwd, pwd );
	cpdCharPassAgain.GetText( sizeof pwd2, pwd2);
	
	int badField = 0;
	if ( '\0' == pwd[0] )
		{
		badField = cpdCharPassEdit;
		cpdCharPass.SelectText();
		}
	else
	if ( '\0' == pwd2[0] )
		{
		badField = cpdCharPass2Edit;
		cpdCharPassAgain.SelectText();
		}
	
	if ( badField )
		{
		GenericError( _(TXTCL_MUST_FILL_FIELD) );
		__Verify_noErr( HIViewSetFocus( GetItem( Item( badField) ),
							kControlEditTextPart, 0 ) );
		return false;
		}
	
	// bad-quality passwords?
	if ( not CheckPasswordQuality( pwd ) )
		{
		badField = cpdCharPassEdit;
		cpdCharPass.SelectText();
		}
	else
	if ( not CheckPasswordQuality( pwd2 ) )
		{
		badField = cpdCharPass2Edit;
		cpdCharPassAgain.SelectText();
		}
	
	if ( badField )
		{
		GenericError( "Insecure Password\n"
					  "Passwords must be 8 to 15 characters long, "
					  "and must include at least one non-letter." );
		__Verify_noErr( HIViewSetFocus( GetItem( Item( badField) ),
							kControlEditTextPart, 0 ) );
		return false;
		}
	
	// both the same?
	if ( strcmp( pwd, pwd2 ) != 0 )
		{
		cpdCharPass.SelectText();
		GenericError( _(TXTCL_PASS_NOT_SAME) );
		return false;
		}
	
	return true;
}

#pragma mark -


/*
**	ProxyDlg::GetNibWindowName()
**	select the release or debug versions of the dialog
*/
CFStringRef
ProxyDlg::GetNibWindowName() const
{
#ifdef DEBUG_VERSION
	return CFSTR("Set IP Addr Devel");
#else
	return GetNibFileName();
#endif
}


/*
**	ProxyDlg::Init()
*/
void
ProxyDlg::Init()
{
	// 'use proxy' is true if the user's desired address matches the
	// client's host address and neither matches the default host address
	int value = 0;
	if ( 0 == strcmp( gPrefsData.pdProxyAddr, gPrefsData.pdHostAddr )
	&&	 0 != strcmp( gPrefsData.pdProxyAddr, kDefaultHostAddrInternet ) )
		{
		value = 1;
		}
	SetValue( Item( proxyUseProxyCheck ), value );
	
	SetText( Item( proxyAddressEdit ), gPrefsData.pdProxyAddr );
	SetText( Item( proxyAddressDefaultText), CFSTR(kDefaultHostAddrInternet) );
	
	// decide which one to show, the editable or the static
	SetupProxy();
	
	SetValue( Item( proxyUseSpecificPortCheck ), not gPrefsData.pdUseArbitrary );
	SetNumber( Item( proxyClientPortEdit ), gPrefsData.pdClientPort );
	
	// make port field visible only if we're NOT letting
	// TCP/IP choose an arbitrary port
	SetVisible( Item( proxyClientPortEdit ), not gPrefsData.pdUseArbitrary );
}


/*
**	ProxyDlg::SetupProxy()
**
**	adjust host address related UI
*/
void
ProxyDlg::SetupProxy() const
{
	bool bUseAltAddr = GetValue( Item( proxyUseProxyCheck ) );
	if ( bUseAltAddr )
		{
		SetVisible( Item(proxyAddressDefaultText), false );
		SetVisible( Item(proxyAddressEdit), true );
		
		SelectText( Item(proxyAddressEdit), 0, 0x3FFF );
		}
	else
		{
		SetVisible( Item(proxyAddressEdit), false );
		SetVisible( Item(proxyAddressDefaultText), true );
		}
}


/*
**	ProxyDlg::DoCommand()
**
**	react to buttons & other UI controls
*/
bool
ProxyDlg::DoCommand( const HICommandExtended& cmd )
{
	bool result = true;
	switch ( cmd.commandID )
		{
		case kHICommandOK:
			if ( Validate() )
				{
				SaveInfo();
				Dismiss( cmd.commandID );
				}
			break;
		
		case cmd_UseProxy:
			SetupProxy();		// update UI based on "use proxy" checkbox
			break;
		
		case cmd_UsePort:
			if ( GetValue( Item(proxyUseSpecificPortCheck) ) )
				{
				// checkbox is on, so let them edit the specific port number
				SetVisible( Item(proxyClientPortEdit), true );
				SelectText( Item(proxyClientPortEdit) );
				}
			else
				// checkbox OFF: TCP/IP picks its own port,
				// so hide the edit field
				SetVisible( Item(proxyClientPortEdit), false );
			break;
		
		case kHICommandRevert:	// aka "Restore Defaults" or "Live Server"
# ifdef DEBUG_VERSION
		case cmd_TestServer:
		case cmd_ThisMachine:
# endif
			{
			CFStringRef addr;
			
# ifdef DEBUG_VERSION
			if ( cmd_TestServer == cmd.commandID )
				addr = CFSTR( kTestServerAddress );
			else
			if ( cmd_ThisMachine == cmd.commandID )
				addr = CFSTR( kSelfAddress );
			else
# endif
				addr = CFSTR( kDefaultHostAddrInternet );
			
			SetText( Item(proxyAddressEdit), addr );
			
			// turn off "Use alternate address" check
			// if they chose "restore defaults"
			SetValue( Item(proxyUseProxyCheck),
				kHICommandRevert == cmd.commandID ? 0 : 1 );
			
			SetupProxy();
			
# ifndef DEBUG_VERSION
			// for non-GM client, "restore defaults" also
			// disables "use specific port" feature
			SetValue( Item( proxyUseSpecificPortCheck ), 0 );
			SetVisible( Item( proxyClientPortEdit ), false );
# endif
			}
			break;
		
		default:
			result = false;
		}
	
	return result;
}


/*
**	ProxyDlg::SaveInfo()
*/
void
ProxyDlg::SaveInfo() const
{
	char addr[ sizeof gPrefsData.pdProxyAddr ];
	bool bUseProxy = GetValue( Item( proxyUseProxyCheck ) );
	
	// fetch the new server "proxy" address
	GetText( Item( proxyAddressEdit ), addr,
		sizeof addr, kCFStringEncodingASCII );
	
	// ensure it includes a port number
	if ( not strrchr( addr, ':' ) )
		strlcat( addr, DEFAULT_PORT, sizeof addr );
	
	// save it in the prefs' proxyAddr field
	StringCopySafe( gPrefsData.pdProxyAddr,
		addr, sizeof gPrefsData.pdHostAddr );
	
	// ... and also in the prefs' _host_ addr field,
	// but only if the checkbox is on
	if ( bUseProxy )
		StringCopySafe( gPrefsData.pdHostAddr,
			addr, sizeof gPrefsData.pdHostAddr );
	else
		{
		// otherwise use the default server address
		StringCopySafe( gPrefsData.pdHostAddr,
			kDefaultHostAddrInternet, sizeof gPrefsData.pdHostAddr );
		}

	// handle specific-port feature
	gPrefsData.pdUseArbitrary = not GetValue( Item(proxyUseSpecificPortCheck) );
	
	bool portOK = false;
	int portNum = GetNumber( Item(proxyClientPortEdit), &portOK );
	if ( not portOK )
		portNum = 0;
	
	gPrefsData.pdClientPort = portNum;
}

#pragma mark -


/*
**	ConnectCommInfo()
**
**	set the playing game flag
*/
DTSError
ConnectCommInfo( char * challenge )
{
	gPlayingGame = true;
	
	DTSError result = ConnectComm( challenge );
	
	gPlayingGame = false;
	SetPlayMenuItems();
	
	return result;
}


/*
**	ExtractJoinName()
**
**	extract the join name
**	either the char name matches the join name
**	-- in which case we copy the source buffer -- or
**	the source matches this pattern "%s (%s)",
**	and we copy the name in the parentheses
*/
void
ExtractJoinName( const char * source, char * dest )
{
	int ch;
	char * ptr = dest;
	const char * limit = dest + kMaxJoinLen - 1;
	const uchar * src = reinterpret_cast<const uchar *>( source );
	
	// first, assume no parens
	for(;;)
		{
		// copy blithely until...
		ch = *src++;
		if ( 0 == ch )		// ... end of string
			{
			*ptr = '\0';
			return;
			}
		if ( '(' == ch )	// ... or we saw open-paren
			{
			ptr = dest;		// prepare for 2nd copy loop below
			break;
			}
		if ( ptr < limit )	// ... but don't overflow buffer
			*ptr++ = ch;
		}
	
	// we only get here if we just saw an open-paren
	for(;;)
		{
		// copy until end-of-string, or close-paren, or overflow
		ch = *src++;
		if ( 0 == ch
		||   ')' == ch
		||   ptr >= limit )
			{
			*ptr = '\0';
			return;
			}
		*ptr++ = ch;
		}
}


/*
**	EncodePassword()
**
**	encode a new password using the old password
*/
char *
EncodePassword( char * dst, const char * newpw, const char * oldpw )
{
	// copy the password
	char encoded[ kMaxPassLen ];
	StringCopySafe( encoded, newpw, sizeof encoded );
	
	// append random data, to pad out to kMaxPassLen
	size_t len = strlen( encoded ) + 1;
	for ( ;  len < sizeof encoded;  ++len )
		encoded[ len ] = GetRandom( 256 );	// may include NULs, which is OK
	
	// encode the new password using the old one as the key
	DTSEncode( encoded, dst, sizeof encoded, oldpw, strlen( oldpw ) );
	
	// return a pointer to just beyond the encoded pwd in the output buffer
	dst += sizeof encoded;
	return dst;
}


/*
**	CheckPasswordQuality()
**
**	check the quality of their password
**	[keep this in sync with server's authoritative definition of quality;
**	cf. server/source/Connect_cls.cp, and elsewhere]
*/
bool
CheckPasswordQuality( const char * password )
{
	//  "To more securely protect your account and characters, "
	//	"passwords must be at least 8 characters, "
	//	"at most 15 characters, "
	//	"and contain at least one non-letter. "
	//	"Please change it now."
	
	// verify length
	size_t len = strlen( password );
	if ( len < 3 || len > 15 )
		return false;
	
	bool bNonAlpha = false;
	for(;;)
		{
		int ch = * (const uchar *) password++;
		if ( 0 == ch )
			break;
		
		// does it contain a character not listed in VALID_PASSWORD_CHARS?
		if ( not strchr( VALID_PASSWORD_CHARS, ch ) )
			return false;
		
		if ( not isalpha( ch ) )
			{
			// the non-letter requirement is met
			bNonAlpha = true;
			break;
			}
		}
	
	return bNonAlpha;	// has at least 1 non-alpha?
}


/*
**	CheckCharNameQuality()
**
**	as above but for CL character names: 3-15 [MacRoman] characters,
**	all from VALID_NAME_CHARS
*/
bool
CheckCharNameQuality( const char * name )
{
	// verify length
	size_t len = strlen( name );
	if ( len < 3 || len > 15 )
		return false;
	
	for (;;)
		{
		int ch = * (const uchar *) name++;
		if ( 0 == ch )
			break;
		
		// does it contain a character not listed in VALID_NAME_CHARS?
		if ( not strchr( VALID_NAME_CHARS, ch ) )
			return false;
		}
	
	return true;
}

/*
**	SendWaitReplyLoop()
**
**	send a message and wait for the reply
**	auto update if indicated
*/
DTSError
SendWaitReplyLoop( char * challenge, Message * msg,
	size_t sz, HIViewRef spinVu /* = nullptr */ )
{
	DTSError result( noErr );
	Message msgsave;
	
	// show the "chasing-arrows" spinner (if we have one)
	if ( spinVu )
		{
		__Verify_noErr( HIViewSetVisible( spinVu, true ) );
		}
	
	// loop to auto update
	for(;;)
		{
		// send the message and wait for the reply
		// without trashing the original message
		// we might need to send it again
		msgsave = *msg;
		result = SendWaitReply( &msgsave, sz, msgsave.msgLogOn.logData );
		
		// bail on any result other than a successful auto-update "error"
		if ( result != kDownloadNewVersionLive )
			break;
		
		// we successfully auto-updated
		// in the process the server stopped talking to us
		// so reconnect
		ExitComm();
		result = ConnectCommInfo( challenge );
		
		// most likely we just changed the version numbers of 
		// the images and/or sounds file
		// it would be a good idea to reset the logon request header
		ResetupLogonRequest( &msg->msgLogOn );
		
		// bail on any error
		if ( result != noErr )
			break;
		}
	
	// stop spinning
	if ( spinVu )
		__Verify_noErr( HIViewSetVisible( spinVu, false ) );
	
	*msg = msgsave;
	return result;
}
