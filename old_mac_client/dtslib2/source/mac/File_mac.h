/*
**	File_dts.h		dtslib2
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

#ifndef FILE_MAC_H
#define FILE_MAC_H

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "Local_dts.h"
#include "File_dts.h"


/*
**	Define DTSFileSpec class
*/
class DTSFileSpecPriv
{
private:
	char			fileName[ 256 ];	// unicodify me!
	UInt32			fileDirectory;
	FSVolumeRefNum	fileVolume;
	
	// constructors, assignment
public:
						DTSFileSpecPriv();
						~DTSFileSpecPriv() {}
						DTSFileSpecPriv( const DTSFileSpecPriv& rhs );
	DTSFileSpecPriv&	operator=( const DTSFileSpecPriv& rhs );
	
	// unicodify these!
	const char *		GetFileName() const { return fileName; }
	void				SetFileName( const char * newName, bool bStillResolved = false );
	void				SetFileName( ConstHFSUniStr255Param, bool bStillResolved = false );
	
	void				SetVolDir( FSVolumeRefNum vol, UInt32 dir )
							{
							fileVolume = vol;
							fileDirectory = dir;
							}
	UInt32				GetDirectory() const { return fileDirectory; }
	FSVolumeRefNum		GetVolume() const { return fileVolume; }
	
	void				SetResolved( bool isResolved )
							{
							mState = isResolved ? fsResolved :
								fileName[0] ? fsPartial : fsUnresolved;
							}
	bool				IsResolved() const { return mState == fsResolved; }
	
	DTSError			CopyToRef( FSRef * outRef, HFSUniStr255 * outName, bool bResolve = true );
	DTSError			CopyToRef( FSRef * outRef, bool bResolve );
	DTSError			CopyFromRef( const FSRef * ref );
	CFURLRef			CreateURL();
	DTSError			SetFromURL( CFURLRef u );
	
private:
	enum RState
		{
		fsUnresolved,		// Filespec status is not known
		fsResolved,			// it definitely points to an object, and the name is empty
		fsPartial			// vol/dir are good; name may or may not point to something
		};
	RState		mState;
};


/*
**	Implementation Routines
*/

	// converts a Mac resource id to a Mac file type via FREF lookup (yuck)
DTSError	GetFileType( int inFREFID, OSType * oFiletype );

DTSError	InitCurVolDir();


// Conversion between DTSFileSpecs and FSRefs
	// The first REQUIRES that the target spec object exist in the file system!
	// The second requires merely that the target's _parent_ directory exist.
DTSError	DTSFileSpec_CopyToFSRef( DTSFileSpec * spec, FSRef * ref, bool bResolveLast = true );
DTSError	DTSFileSpec_CopyToFSRef( DTSFileSpec * spec, FSRef * parRef,
				HFSUniStr255 * name, bool bResolveLast = true );
DTSError	DTSFileSpec_CopyFromFSRef( DTSFileSpec * spec, const FSRef * ref );
DTSError	DTSFileSpec_Set( DTSFileSpec * spec, const FSRef * parDir, CFStringRef leafName );

// likewise, conversions between DTSFileSpecs and CFURLRefs.
CFURLRef	DTSFileSpec_CreateURL( const DTSFileSpec * spec );
DTSError	DTSFileSpec_SetFromURL( DTSFileSpec * oSpec, CFURLRef inURL );

// useful utilities
FILE *		fopen( const FSRef * parDir, CFStringRef fname, const char * mode );
FILE *		fopen( CFURLRef url, const char * mode );


// This is meant to be a mixin class -- you can't just allocate one of these,
// standalone, on the stack, and blithely fire it off, 'cos the object will go away
// before the dialog returns, which will lead to tears.
//
// Instead, subclass this to provide the behavior you want, then mix your subclass into
// your DTSWindow subclasses, or what-have-you.
class NavSrvHandler
{
public:
	virtual void	Action( NavCBRec * parms ) = 0;
	virtual void	Event( NavEventCallbackMessage sel, NavCBRec * parms );
	virtual bool	Filter( AEDesc * item, const NavFileOrFolderInfo * info,
							NavFilterModes mode );
	virtual bool	Preview( NavCBRec * parms );
	virtual void	Terminate( NavCBRec * parms );
	virtual CFArrayRef	CopyFilterTypeIdentifiers() const;
	
public:
	NavSrvHandler() {}
	virtual ~NavSrvHandler() {}

private:
	NavSrvHandler( const NavSrvHandler& );	// no copies
	NavSrvHandler& operator=( const NavSrvHandler& );
};

// Utility interfaces for each kind of NavSvcs3 dialog.
OSStatus	AskGetFile( NavSrvHandler&, const NavDialogCreationOptions& );
OSStatus	AskPutFile( NavSrvHandler&, OSType, OSType, const NavDialogCreationOptions& );
OSStatus	AskChooseFile( NavSrvHandler&, const NavDialogCreationOptions& );
OSStatus	AskChooseFolder( NavSrvHandler&, const NavDialogCreationOptions& );
OSStatus	AskChooseVolume( NavSrvHandler&, const NavDialogCreationOptions& );
OSStatus	AskChooseObject( NavSrvHandler&, const NavDialogCreationOptions& );
OSStatus	AskSaveChanges( NavSrvHandler&, NavAskSaveChangesAction,
							const NavDialogCreationOptions& );
OSStatus	AskDiscardChanges( NavSrvHandler&, const NavDialogCreationOptions& );
OSStatus	AskReviewDocuments( NavSrvHandler&, const NavDialogCreationOptions&, UInt32 );
OSStatus	AskCreateNewFolder( NavSrvHandler&, const NavDialogCreationOptions& );

#endif  // FILE_MAC_H
