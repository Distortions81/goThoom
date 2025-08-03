/*
**	TextCmdList_cl.cp		ClanLord
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

#include "Commands_cl.h"
#include "Macros_cl.h"
#include "TextCmdList_cl.h"


CTextCmdList * gCmdList = NEW_TAG("CTextCmdList") CTextCmdList;


CTextCmdList::CTextCmdList() :
	mPending(),
	mCacheMarker(),
	mLastID()
{
	// Initialize queues
	mReadyQ.qTail = mReadyQ.qHead = nullptr;	mReadyQ.qFlags = 0;
	mFreeQ.qTail  = mFreeQ.qHead  = nullptr;	mFreeQ.qFlags  = 0;
	mCacheQ.qTail = mCacheQ.qHead = nullptr;	mCacheQ.qFlags = 0;
	
	Flush();
}


/*
CTextCmdList::~CTextCmdList()
{
}
*/


//
// Flush the buffers, put them back in the free queue,
//
void
CTextCmdList::Flush()
{ 
	STextCmd * b;
	while ( (b = reinterpret_cast<STextCmd *>( mReadyQ.qHead )) != nullptr )
		::Dequeue( &b->elem, &mReadyQ );
	while ( (b = reinterpret_cast<STextCmd *>( mFreeQ.qHead  )) != nullptr )
		::Dequeue( reinterpret_cast<QElemPtr>( b ), &mFreeQ  );
	while ( (b = reinterpret_cast<STextCmd *>( mCacheQ.qHead )) != nullptr )
		::Dequeue( reinterpret_cast<QElemPtr>( b ), &mCacheQ );
	
	// Install all our buffers in the Free Queue
	for ( int ii = 0; ii < max_Commands; ++ii )
		{
		bzero( &mCmds[ii], sizeof mCmds[ii] );
		mCmds[ii].mOwner 	= this;
//		mCmds[ii].mCache	= false;
		mCmds[ii].mSend		= true;
//		mCmds[ii].mID		= 0;
//		mCmds[ii].mText[0]	= '\0';
		
		::Enqueue( &mCmds[ii].elem, &mFreeQ );
		}
	mPending = nullptr;
}


//
// Cache a command
//
void
CTextCmdList::CacheCmd( STextCmd * cacheCmd )
{
	if ( cacheCmd->mCache )
		{
		// look for it in the cache
		STextCmd * cmd = reinterpret_cast<STextCmd *>( mCacheQ.qHead );
		for ( ; cmd; cmd = reinterpret_cast<STextCmd *>( cmd->elem.qLink ) )
			{
			if ( not std::strcmp( cmd->mText, cacheCmd->mText ) )
				break;
			}
		// if it isn't in the cache, go ahead and cache it; else,
		// bring it to front of the cache queue, and free the pending command
		if ( not cmd )
			::Enqueue( &cacheCmd->elem, &mCacheQ );
		else
			{
			::Dequeue( &cmd->elem, &mCacheQ );
			::Enqueue( &cmd->elem, &mCacheQ );
			::Enqueue( &cacheCmd->elem, &mFreeQ );
			}
		
		// Set cache marker to NULL	(which means no looking in the cache has been done)
		mCacheMarker = nullptr;
		}
	else
		::Enqueue( &cacheCmd->elem, &mFreeQ );
}


//
// Add a command to the ready queue
// return true if it went correctly
//
bool
CTextCmdList::SendCmd( bool inCache, const char * inCmd, bool doSend )
{
	if ( not IsSendCmdAvail() )
		return false;
	
	STextCmd * cmd = nullptr;
	
	// try to pluck a spare cmd off of the free queue
	if ( not cmd
	&&	 ( cmd = reinterpret_cast<STextCmd *>( mFreeQ.qHead )) != nullptr )
		{
		::Dequeue( &cmd->elem, &mFreeQ );
		}
	
	// failing that, try to pluck one off of the cache queue
	if ( not cmd
	&&	 (cmd = reinterpret_cast<STextCmd *>( mCacheQ.qHead )) != nullptr )
		{
		::Dequeue( &cmd->elem, &mCacheQ );
		}
	
	if ( not cmd )		// overkill, 'cos IsSendCmdAvail() would have spotted this already
		return false;
	
#if CLIENT_COMMANDS	
    // If it is handled locally, don't send it to the server
    
    cmd->mClientCommand = ClientCommand::HandleCommand( inCmd );
    // don't send client commands
	if ( cmd->mClientCommand != ClientCommand::kNotHandled )
		doSend = false;
#endif
	
	cmd->mSend  = doSend;
	cmd->mCache = inCache;
	cmd->mID 	= 0;
	StringCopySafe( cmd->mText, inCmd, sizeof cmd->mText );
	
	::Enqueue( &cmd->elem, &mReadyQ );
	
	return true;
}


//
// If no command is pending, and one is ready, return its data
// and mark that command as pending
//
bool
CTextCmdList::GetCmdToSend( ulong &ioID, char * ioCmd )
{
	ioCmd[ 0 ] = '\0';
	
	if ( IsCmdPending() )
		return false;
	
	// Let continuing macros execute
	ContinueMacroExecution();
	
	STextCmd * cmd = reinterpret_cast<STextCmd *>( mReadyQ.qHead );
	
	if ( not cmd )
		return false;
	
	bool stopLooking = false;
	
	// Move past a command we aren't supposed to send, possibly caching it
	if ( not cmd->mSend )
		{
		::Dequeue( &cmd->elem, &mReadyQ );
		CacheCmd( cmd );
		return stopLooking ? false : GetCmdToSend( ioID, ioCmd );
		}
	
	const char * cmdLine = cmd->mText;
	const char * nextpart = std::strchr( cmdLine, '\r' );
	size_t l = nextpart ? size_t(nextpart - cmdLine) : std::strlen( cmdLine );
	
	if ( l )
		{
		// we got something to send after all
		std::strncpy( ioCmd, cmdLine, l );
		ioCmd[ l ] = '\0';
		}
	else
		{
		// command is empty, try for some more..
		::Dequeue( &cmd->elem, &mReadyQ );
		::Enqueue( &cmd->elem, &mFreeQ );
		return stopLooking ? false : GetCmdToSend( ioID, ioCmd );
		}
	
	//
	// Now, if it was a resend, reset the command counter
	// to that command's counter, else we increment it
	//
	if ( cmd->mID )
		ioID = cmd->mID;
	else
		{
		// OK, after some tests, I *know* that sending command 256
		// gets you an ack == 0 !!! Don't know why, but let's make sure it works
		// (the real reason is, only 8 bits of the ack number are sent in a packet)
		ioID = (ioID + 1) & 0xFF;
		if ( 0 == ioID )
			++ioID;
		cmd->mID = ioID;
		}
	
	::Dequeue( &cmd->elem, &mReadyQ );
	mPending = cmd;
	return true;
}


//
// We had a command pending, and now it's done
// so free it, or cache it
//
bool
CTextCmdList::CmdDone( ulong inID )
{
	mLastID = inID;
	if ( not IsCmdPending() )
		return false;
	
	//
	// If the ack we receive is wrong, reinstall the
	// pending command into the ready queue, with the same number.
	// it seems we receive an ack & 0xff for our commands.
	// that's strange... (not really; see above.)
	//
	if ( (mPending->mID & 0x0FF) != (inID & 0x0FF) )
		{
		// not sure why this part isn't just:
		//	::Enqueue( &mPending->elem, &mReadyQ );
		
		mPending->elem.qLink = mReadyQ.qHead;
		mReadyQ.qHead = &mPending->elem;
		
		// if the queue was previously empty, then we better update the
		// queue tail to point to this newly added item.
		if ( not mReadyQ.qTail )
			mReadyQ.qTail = &mPending->elem;
		
		// ShowMessage( "Resend cmd %d", mPending->mID );
		mPending = nullptr;
		return false;
		}
	
	//
	// Pending command is done: cache it maybe, or just free it
	// 
	if ( not mPending )
		return false;
	
	//
	// Handle 'standard' multiline macros
	//
	char * nextpart = std::strchr( mPending->mText, '\r' );
	if ( nextpart )
		{
		*nextpart = '\0';
		++nextpart;
		size_t l = std::strlen( nextpart );
		if ( l > 0 )
			{
			std::memmove( mPending->mText, nextpart, l + 1 );
			
			// requeue it :)
			mPending->elem.qLink = mReadyQ.qHead;
			mReadyQ.qHead = &mPending->elem;
			
			// see above
			if ( not mReadyQ.qTail )
				mReadyQ.qTail = &mPending->elem;
			
			mPending->mID = 0;
			mPending = nullptr;
			return true;
			}
		}
	
	mPending->mID = 0;
	CacheCmd( mPending );
	mPending = nullptr;
	return true;
}


/*
**	CTextCmdList::GetCachedCmd()
**
**	handle the command history
*/
bool
CTextCmdList::GetCachedCmd( short inWhat, char * ioCmd )
{
	if ( not mCacheQ.qHead )
		return false;
	
	STextCmd * cmd = nullptr;
	if ( mCacheQ.qHead == mCacheQ.qTail )
		cmd = reinterpret_cast<STextCmd *>( mCacheQ.qHead );
	else 
		{
		// Look for a cmd with different text (there HAS to be one or this is an infinite loop!)
		do
			{
			cmd = mCacheMarker;
			if ( inWhat == cmd_First )
				{
				// Move backward one cmd
				if ( not cmd
				||	 cmd == reinterpret_cast<STextCmd *>( mCacheQ.qHead ) )
					{
					// loop around to the end
					cmd = reinterpret_cast<STextCmd *>( mCacheQ.qTail );
					}
				else
					{
					cmd = reinterpret_cast<STextCmd *>( mCacheQ.qHead );
					while ( cmd->elem.qLink
					&&	   reinterpret_cast<STextCmd *>(cmd->elem.qLink ) != mCacheMarker )
						{
						cmd = reinterpret_cast<STextCmd *>( cmd->elem.qLink );
						}
					}
				mCacheMarker = cmd;
				}
			else
				{
				// Move forward one command
				if ( not cmd
				||	 not cmd->elem.qLink )
					{
					// loop back to start
					cmd = reinterpret_cast<STextCmd *>( mCacheQ.qHead );
					}
				else
					cmd = reinterpret_cast<STextCmd *>( cmd->elem.qLink );
				
				mCacheMarker = cmd;
				}
			
			} while ( not std::strcmp( ioCmd, cmd->mText ) );
		}
	if ( cmd )
		std::strcpy( ioCmd, cmd->mText );
	
	return cmd != nullptr;
}


//
// this is probably overkill, but I'm paranoid..
//
void
CTextCmdList::DebugReport() const
{
#ifdef DEBUG_VERSION
	
	// count up all the known queue entries
	int	ready = 0;
	STextCmd * cmd;
	for ( cmd = reinterpret_cast<STextCmd *>( mReadyQ.qHead );
		  cmd;
		  cmd = reinterpret_cast<STextCmd *>( cmd->elem.qLink ) )
		{
		++ready;
		}
	
	int	cache = 0;
	for ( cmd = reinterpret_cast<STextCmd *>( mCacheQ.qHead );
		  cmd;
		  cmd = reinterpret_cast<STextCmd *>( cmd->elem.qLink ) )
		{
		++cache;
		}
	
	int	free = 0;
	for ( cmd = reinterpret_cast<STextCmd *>( mFreeQ.qHead );
		  cmd;
		  cmd = reinterpret_cast<STextCmd *>( cmd->elem.qLink ) )
		{
		++free;
		}
	
	int	pending	= mPending ? 0 : 1;
	
	ulong wantID = 0;
	if ( mPending )
		wantID = mPending->mID;
	else
	if ( mReadyQ.qHead )
		{
		cmd = reinterpret_cast<STextCmd *>( mReadyQ.qHead );
		if ( cmd->mID )
			wantID = cmd->mID;
		}
	
	// now see if the books are in balance
	int	total = ( ready + free + cache + pending );
	ShowMessage( BULLET " %d ready/%d free/%d cache/%d pending = %d/%d: %s",
			 ready, free, cache, pending, total, max_Commands,
			 total == max_Commands ? "fine" : "OUCH!" );
	if ( wantID )
		ShowMessage( BULLET " Last ack %u, want %u", (uint) mLastID, (uint) wantID );
	
#endif	// DEBUG_VERSION
}


