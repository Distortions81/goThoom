/*
**	ResourceIDs.h		ClanLord
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

/*
**	Various constants and definitions to be shared by both
**	the C++ and Rez compilers.
**
**	NOTE: This file is #include'd by both ClanLord.h and ClanLord.r
**	Don't put anything in here that Rez will barf on.
**	(But if you must, see COMPILE_FOR_REZ, below.)
*/

#ifndef RESOURCEIDS_H
#define RESOURCEIDS_H

// You can test this macro to see which compiler is at work, C or Rez:
#if defined( rez ) || defined( derez ) || defined( REZ ) || defined( DEREZ )
# define COMPILE_FOR_REZ		1
#else
# define COMPILE_FOR_REZ		0
#endif	// rez/derez

// are menus defined in a nib file, or in 'MENU' etc. resources?
#define USE_MBAR_NIB			1

/*
**	Constants for the Menu Bar
*/
enum
	{
	rMenuBarID = 128,
	
	// main menus
	rAppleMenuID = 128,
	rFileMenuID,
	rEditMenuID,
	rOptionsMenuID,
	rDebugMenuID,
	rCommandMenuID,
	rGodMenuID,
	
	// popups
	rNightLevelPopupMenuID,
	rBalloonsPopupMenuID,
	
	// submenus
	rLabelSubmenuID,	// valid submenu id's are in the range of 1-235
	rURLMenuID = 255
	};


/*
**	Menu item codes.
**	Rez can't handle parameter substitution in preprocessor macros
**	so we have to use a workaround for MakeLong()
*/
enum
	{
	// the about menu
#if COMPILE_FOR_REZ
	mAbout = (rAppleMenuID << 16) + 1,
#else
	mAbout = MakeLong( rAppleMenuID, 1 ),
#endif
	
	// the file menu
#if COMPILE_FOR_REZ
	mCharManager = (rFileMenuID << 16) + 1,
#else
	mCharManager = MakeLong( rFileMenuID, 1 ),
#endif
	mSelectChar,
	mJoinGame,
	mDisconnect,
	mFileDisabled1,
	mViewMovie,
	mRecordMovie,
	
	// the edit menu
#if COMPILE_FOR_REZ
	mUndo = (rEditMenuID << 16) + 1,
#else
	mUndo = MakeLong( rEditMenuID, 1 ),
#endif
	mEditDisabled1,
	mCut,
	mCopy,
	mPaste,
	mClear,
#if USE_MBAR_NIB
	mSelectAll,
	mEditDisabled2,
#else
	mEditDisabled2,
	mSelectAll,
#endif
	mFind,
	mFindAgain,
	
	// the options menu
#if COMPILE_FOR_REZ
	mSound = (rOptionsMenuID << 16) + 1,
#else
	mSound = MakeLong( rOptionsMenuID, 1 ),
#endif
	mMusicPlay,
	mOptionsDisabled1,
	mTextColors,
	mReloadMacros,
	mSaveTextLog,
	mNetworkStats,
	mOptionsDisabled2,
	mGameWindow,
	mTextWindow,
	mInvenWindow,
	mPlayersWindow,
	
	// the command menu
#if COMPILE_FOR_REZ
	mInfo = (rCommandMenuID << 16) + 1,
#else
	mInfo = MakeLong( rCommandMenuID, 1 ),
#endif
	mPull,
	mPush,
	mCommandDisabled1,
	mKarma,
	mThank,
	mCommandDisabled2,
	mCurse,
	mCommandDisabled3,
	mShare,
	mUnshare,
	mCommandDisabled4,
	mBefriend,
	mBlock,
	mIgnore,
	mCommandDisabled5,
	mEquipItem,
	mUseItem,
	
	// the label submenu
#if COMPILE_FOR_REZ
	mNoLabel = (rLabelSubmenuID << 16) + 1,
#else
	mNoLabel = MakeLong( rLabelSubmenuID, 1 ),
#endif
	mRedLabel,
	mOrangeLabel,
	mGreenLabel,
	mBlueLabel,
	mPurpleLabel,

	// the god menu
#if COMPILE_FOR_REZ
	mShowMonsterNames = (rGodMenuID << 16) + 1,
#else
	mShowMonsterNames = MakeLong( rGodMenuID, 1 ),
#endif
	mGodDisabled1,
	mAccount,
	mSet,
	mGodDisabled2,
	mStats,
	mRanks,
	mGodDisabled3,
	mGoto,
	mWhere,
	mPunt,
	mTricorder,
	mFollow,
	
	// the debug menu
#if COMPILE_FOR_REZ
	mShowFrameTime = (rDebugMenuID << 16) + 1,
#else
	mShowFrameTime = MakeLong( rDebugMenuID, 1 ),
#endif
	mSaveMessageLog,
	mShowDrawTime,
	mShowImageCount,
	mFlushCaches,
	mSetIPAddress,
	mFastBlitters,
	mDrawRateControls,
	mDumpBlocks,
	mPlayerFileStats,
	mSetVersionNumbers,
	
#if COMPILE_FOR_REZ
	mClientHelpURL = (-16490 << 16) + 1		// kHMHelpMenuID
#else
	mClientHelpURL = MakeLong( kHMHelpMenuID, 1 )
#endif
	};


#if 0  // supplanted by HIDialogs
/*
**	Dialog IDs
*/
enum
	{
	// dialogs
	rPolicyDlgID = 128,
	rDeleteCharacterDlgID,
	rProxyDlgID,				// 130
	rMessageDlgID,
	rControlsDlgID,
	rCreateCharacterDlgID,
	rSelectCharacterDlgID,
	rEnterPasswordDlgID,		// 135
	rChangePasswordDlgID,
	rSelectAccountDlgID,
	rCharacterManagerDlgID,
	rBetaPasswordDlgID,
	rConnectCancelDlgID,		// 140
	rAdvancedPrefsDlgID,	
	rFindTextDlgID,
	rColorChoiceDlgID,
	rDrawRateControlsDlgID,
	rNetworkStatsDlgID,			// 145
	rVersionNumbersDlgID
	};
#endif  // 0

#endif	// RESOURCEIDS_H

