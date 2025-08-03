/*
**	TextCmdList_cl.h		ClanLord
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

#ifndef TEXTCMDLIST_CL_H
#define TEXTCMDLIST_CL_H


#include "ClanLord.h"

class CTextCmdList	
{
public:
	struct STextCmd
		{
		QElem				elem;				// Reserved
		CTextCmdList *		mOwner;				// Owner
		ulong				mID;				// command ID
		bool				mSend;				// should it be sent?
		bool				mCache;				// should be cached and/or sent
		short             	mClientCommand;     // is this a delayed client command?
		char				mText[ kPlayerInputStrLen ];
		};
	
	enum
		{
		max_Cached			= 10,
		max_Pending			= 5,
		max_Commands		= max_Cached + max_Pending,
		
		// for GetCachedCmd
		cmd_Last			= 0,
		cmd_First
		};
	
	
							CTextCmdList();
	/*virtual*/				~CTextCmdList() {}
	
	void					Flush();
	void					CacheCmd( STextCmd * cacheCmd );
	
	bool					SendCmd( bool inCache, const char * inCmd, bool doSend = true );
	bool					GetCmdToSend( ulong& ioID, char * ioCmd );
	bool					CmdDone( ulong inID );
	
	bool					IsSendCmdAvail() const
		{ 	return mFreeQ.qHead != nullptr || mCacheQ.qHead != nullptr;	}
	bool					IsCmdPending() const
		{ 	return mPending != nullptr; }
	bool					NoneReady() const
		{ 	return mReadyQ.qHead == nullptr && mPending == nullptr; }
	
	bool					GetCachedCmd( short inWhat, char * ioCmd );
	
	void					DebugReport() const;
	
protected:
	STextCmd				mCmds[ max_Commands ];
	STextCmd *				mPending;
	STextCmd *				mCacheMarker;
	QHdr					mReadyQ;
	QHdr					mFreeQ;
	QHdr					mCacheQ;
	ulong					mLastID;

private:
							CTextCmdList( const CTextCmdList& );
	CTextCmdList&			operator=( const CTextCmdList& );
};

extern CTextCmdList *		gCmdList;

#endif	// TEXTCMDLIST_CL_H

