/*
**	Comm_cl.cp		ClanLord
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

#include "ClanLord.h"
#include "KeychainUtils.h"
#include "LaunchURL_cl.h"
#include "Movie_cl.h"


/*
**	Entry Routines
*/
/*
DTSError	InitReadMovie();
DTSError	InitComm();
void		ExitComm();
void		DoComm();
DTSError	ConnectComm( char challenge[kChallengeLen] );
DTSError	SendWaitReply( Message * msg, size_t size, char * error );
char *		AnswerChallenge( char * dst, const char * password, const char * challenge );
void		SetupLogonRequest( LogOn * logon, ushort messagetag );
void		ResetupLogonRequest( LogOn * logon );
*/


/*
**	Internal Definitions
*/
enum
	{
	kLogOnTimeout		= 60 * 60,
	kLogOnResetTimeout	= 0x0001,
	kLogOnSend			= 0x0002,
	kLogOnGetName		= 0x0004
	};


class CLNetChannel : public DTSNetChannel
{
public:
	// overloaded routines
	virtual DTSError	ConnectCallBack( const char * status );
};

const int kCallBackDelay		= 3 * 60;


/*
**	Internal Routines
*/
static DTSError		FindServer();
static DTSError		WaitForChallenge( char * challenge );
static DTSError		GetChallenge( char * challenge );
static DTSError		LogOnToHost( char * challenge );
static DTSError		LogOnToHostLoop( char * challenge );
static DTSError		SendLogOn( char * challenge );
static DTSError		GetLogOnResponse( uint * flags );
static DTSError		Handle1Comm();
static void			HandleDrawStateMsg();
static void			SendInput();
static void			ShowDisconMsgs();
static DTSError		SendIdentifiers();
static char *		GetRealLanguage( char * ptr, size_t sz );
static char *		GetEthernetAddress( char * ptr );
static char *		GetUserName( char * ptr, size_t sz );
static char *		GetMachineName( char * ptr, size_t sz );
static char *		GetBootVolumeName( char * ptr, size_t sz );


/*
**	Internal Variables
*/
static CLNetChannel		gNetChannel;
static DataSpool		gMsgSpoolA;
static DataSpool		gMsgSpoolB;
static DataSpool *		gNewMsgSpool;
static ulong			gCallBackTime;


/*
**	InitReadMovie()
**
**	Initialize client to view movie file, don't do network stuff.
*/
DTSError
InitReadMovie()
{
	FlushCaches();

	// clear the error code
	// because the caller looks at it
	DTSError result = noErr;
	ClearErrorCode();
	
	// allocate the message spools
	if ( noErr == result )
		result = gMsgSpoolA.Init( sizeof(Message) );
	if ( noErr == result )
		result = gMsgSpoolB.Init( sizeof(Message) );
	if ( noErr == result )
		{
		gDSSpool     	= nullptr;
		gNewMsgSpool 	= &gMsgSpoolA;
		gAckFrame    	= 0;
		gResendFrame	= 0;	// DONT put at -1. we don't need any resend frames!
		}
	
	return result;	
}


/*
**	InitComm()
**
**	Initialize communications with the host.
*/
DTSError
InitComm()
{
	char challenge[ kChallengeLen ];
	
	// clear the error code
	// because the caller looks at it
	DTSError result = noErr;
	ClearErrorCode();
	
	// make sure we have a character name and password
//	if ( noErr == result )
		{
		// attempt to retrieve from keychain
		if ( gPrefsData.pdCharName[0] )
			{
			KeychainAccess::Get( kPassType_Character, gPrefsData.pdCharName,
				gPrefsData.pdCharPass, sizeof gPrefsData.pdCharPass );
			}
		
		if ( 0 == gPrefsData.pdCharName[0]
		||   0 == gPrefsData.pdCharPass[0] )
			{
			result = SelectCharacter();
			}
		}
	// make sure we have an address for the host
	if ( noErr == result )
		{
		if ( 0 == gPrefsData.pdHostAddr[0] )
			{
#ifdef DEBUG_VERSION
			result = GetHostAddr();
#else
			GenericError( "%s", _(TXTCL_PREF_FILE_PROBLEM) );
			result = -1;
#endif
			}
		}
	// allocate the message spools
	if ( noErr == result )
		result = gMsgSpoolA.Init( sizeof(Message) );
	if ( noErr == result )
		result = gMsgSpoolB.Init( sizeof(Message) );
	
	// connect to the host
	if ( noErr == result )
		result = ConnectComm( challenge );
	
	// log on
	if ( noErr == result )
		result = LogOnToHost( challenge );
	if ( noErr == result )
		{
		gDSSpool     = nullptr;
		gNewMsgSpool = &gMsgSpoolA;
		gAckFrame    =  0;
		gResendFrame = -1;
		gNumFrames   =  0;
		gLostFrames  =  0;
		}
	
	// close the connection for an error
	if ( result != noErr )
		ExitComm();
	
	// show an error message for hard errors
	if ( result < noErr )
		ShowMessage( _(TXTCL_FAILED_INIT_COMM), result );
	
	return result;
}


/*
**	ConnectComm()
**
**	connect to the host
*/
DTSError
ConnectComm( char challenge[ kChallengeLen ] )
{
	// initialize the network port with our name
	DTSError result = DTSInitNetwork();
	
	// find the server
	if ( noErr == result )
		result = FindServer();
	
	// send our identifiers
	if ( noErr == result )
		result = SendIdentifiers();
	
	// wait for the challenge
	if ( noErr == result )
		result = WaitForChallenge( challenge );
	
	return result;
}


/*
**	ExitComm()
**
**	Close communications with the host.
*/
void
ExitComm()
{
	gDSSpool = nullptr;
	
	gNetChannel.Close();
	
	DTSExitNetwork();
	
#ifdef DEBUG_VERSION
//	ShowMessage( "* ExitComm" );
#endif
}


/*
**	DoComm()
**
**	Wait for the host to send us a message.
**	Save the data and redraw the window.
*/
void
DoComm()
{
	// clear the draw state spool
	// so we know if we received a drawstate message
	gDSSpool = nullptr;
	
	DTSError result = noErr;
	
	// handle all of the communications from the server
	while ( noErr == result )
		result = Handle1Comm();
	
	// bail if we did not receive a drawstate message
	DataSpool * ds = gDSSpool;
	if ( not ds )
		return;
	
	if ( not CCLMovie::IsReading() ) // only if on a real server
		{
		// See if there is a command request from the players list.
		RequestPlayersData();
		
		// respond to all drawstate messages
		SendInput();
		}
	
	// redraw the game window
	RedrawCLWindow();
}


/*
**	SetupLogonRequest()
**
**	centralize the task of filling out a LogOn request
**	(makes it easier to twiddle the version numbers, when
**	testing the auto-download code.)
*/
void
SetupLogonRequest( LogOn * logon, uint messagetag )
{
	logon->logMsgTag = NativeToBigEndian( uint16_t( messagetag ) );
	ResetupLogonRequest( logon );
}


/*
**	ResetupLogonRequest()
**
**	skip the tag part
*/
void
ResetupLogonRequest( LogOn * logon )
{
	logon->logResult        = NativeToBigEndian( noErr );
	logon->logClientVersion = NativeToBigEndian( gPrefsData.pdClientVersion );
	logon->logImagesVersion = NativeToBigEndian( gImagesVersion );
	logon->logSoundsVersion = NativeToBigEndian( gSoundsVersion );
}


/*
**	FindServer()
**
**	Find a clan lord server.
*/
DTSError
FindServer()
{
	// attempt to connect to the host
	bool bNeedHostInfo = false;
	DTSError result;
	for (;;)
		{
		// get host address from the user if we need to
		if ( bNeedHostInfo )
			{
#ifdef DEBUG_VERSION
			result = GetHostAddr();
			if ( result != noErr )
				break;
#else
			GenericError( "%s", _(TXTCL_SERVER_UNREACHABLE) );
			result = -1;
			break;
#endif
			}
		
		// find the host and try again
		gCallBackTime = GetFrameCounter() + kCallBackDelay;
		ushort clientport = 0;
		if ( not gPrefsData.pdUseArbitrary )
			clientport = gPrefsData.pdClientPort;
		result = gNetChannel.ConnectHost( gPrefsData.pdHostAddr, clientport );
		if ( noErr == result )
			break;
		
		// allow the Quit menu item to exit this loop
		if ( not gPlayingGame  ||  gDoneFlag )
			break;
		
		// try again
		ShowMessage( _(TXTCL_FAILED_FIND_SERVER), result, gPrefsData.pdHostAddr );
		bNeedHostInfo = true;
		}
	
	return result;
}


/*
**	WaitForChallenge()
**
**	wait for the challenge message to arrive from the server
*/
DTSError
WaitForChallenge( char * challenge )
{
	gConnectGame = true;
	SetPlayMenuItems();
	
	// set the timeout
	ulong thisTime = GetFrameCounter();
	ulong timeout = thisTime + kLogOnTimeout;
	
	// wait
	DTSError result;
	for (;;)
		{
		// run the application
		RunApp();
		
		// check if the user done gone and quit
		if ( gDoneFlag )
			{
			result = kResultCancel;
			break;
			}
		
		// check timeout
		thisTime = GetFrameCounter();
		if ( thisTime > timeout )
			{
			ShowMsgDlg( _(TXTCL_SERVER_NO_RESPONSE) );
			result = -1;
			break;
			}
		
		// get the log on response
		// stop if we succeeded or there was an error
		result = GetChallenge( challenge );
		if ( result <= noErr )
			break;
		}
	
	gConnectGame = false;
	SetPlayMenuItems();
	
	return result;
}


/*
**	GetChallenge()
**
**	get the challenge from the channel
*/
DTSError
GetChallenge( char * challenge )
{
	// read the challenge
	LogOn response;
	size_t size = sizeof response;
	DTSError result = gNetChannel.Read( &response, &size );
	
	// bail if we are still waiting for a message
	if ( result > 0 )
		return +1;
	
	// bail if we got an error
	if ( result < 0 )
		{
		ShowMessage( "* Log on read failed %d.", (int) result );
		return result;
		}
	
	// okay we got a message from the server
	result = BigToNativeEndian( response.logMsgTag );
	// is it a challenge message?
	if ( result != kMsgChallenge )
		{
		// not a challenge response
		// we are confused
		// try again
		ShowMessage( "* Expecting Challenge message from host got %d", (int) result );
		return +1;
		}
	
	// check the challenge result
	result = BigToNativeEndian( response.logResult );
	switch ( result )
		{
		// success!
		case noErr:
			memcpy( challenge, response.logData, kChallengeLen );
			return noErr;
		
		// errors we cannot recover from
		case kIncompatibleVersions:
			{
			ShowMsgDlg( response.logData );
			
			char buff[ 256 ];
			snprintf( buff, sizeof buff,
				_(TXTCL_UPDATE_FROM_WEB),
				kClanLordURL );
			ShowInfoText( buff );
			
			return -1;
			}
			break;
		
		// special case if we need to download a new version of the client
		case kDownloadNewVersionTest:
		case kDownloadNewVersionLive:
			result = AutoUpdate( &response );
			return result;
		
		default:
			ShowMsgDlg( response.logData );
		}
		return -1;
}


/*
**	LogOnToHost()
**
**	Log into the server.
*/
DTSError
LogOnToHost( char * challenge )
{
	// loop handling auto update errors
	for (;;)
		{
		// log on to the server
		DTSError result = LogOnToHostLoop( challenge );
		
		// any result other than a successful download of new data is an error
		if ( result != kDownloadNewVersionLive )
			return result;
		
		// the server bailed on us
		// catch up on the disconnection and reconnect
		ExitComm();
		result = ConnectComm( challenge );
		if ( result != noErr )
			return result;
		}
}


/*
**	LogOnToHostLoop()
**
**	Log into the server.
*/
DTSError
LogOnToHostLoop( char * challenge )
{
	gConnectGame = true;
	SetPlayMenuItems();
	
	DTSError result = noErr;
	
	ulong timeout = 0;
	uint flags = kLogOnResetTimeout | kLogOnSend;
	
	for (;;)
		{
		// run the application
		RunApp();
		
		// check if the user quit
		if ( gDoneFlag )
			{
			result = kResultCancel;
			break;
			}
		
		// check if we need to get a name
		if ( flags & kLogOnGetName )
			{
			flags &= ~kLogOnGetName;
			result = SelectCharacter();
			if ( result != noErr )
				break;
			}
		
		// check if we need to send a log on message
		if ( flags & kLogOnSend )
			{
			flags &= ~kLogOnSend;
			result = SendLogOn( challenge );
			if ( result != noErr )
				break;
			}
		
		// check timeout
		ulong thisTime = GetFrameCounter();
		if ( flags & kLogOnResetTimeout )
			timeout = thisTime + kLogOnTimeout;
		
		if ( thisTime > timeout )
			{
			ShowMsgDlg( _(TXTCL_SERVER_NO_RESPONSE) );
			result = -1;
			break;
			}
		
		// get the log on response
		// stop if we succeeded or there was an error
		result = GetLogOnResponse( &flags );
		if ( result <= noErr )
			break;
		}
	
	gConnectGame = false;
	SetPlayMenuItems();
	
	return result;
}


/*
**	SendLogOn()
**
**	send the log on message
*/
DTSError
SendLogOn( char * challenge )
{
	LogOn logon;
	SetupLogonRequest( &logon, kMsgLogOn );
	
	char * ptr = logon.logData;
	ptr = StringCopyDst( ptr, gPrefsData.pdCharName );
	ptr = AnswerChallenge( ptr, gPrefsData.pdCharPass, challenge );
	
	size_t size = ptr - logon.logData;
	SimpleEncrypt( logon.logData, size );
	
	size = ptr - (char *) &logon;
	DTSError result = gNetChannel.Write( &logon, size, kNetWriteReliable );
	
	return result;
}


/*
**	GetLogOnResponse()
**
**	return noErr to indicate success
**	return a negative error result
**	return a positive number and set the flags to try again
*/
DTSError
GetLogOnResponse( uint * flags )
{
	// read the logon message
	LogOn response;
	size_t size = sizeof response;
	DTSError result = gNetChannel.Read( &response, &size );
	
	// bail if we are still waiting for a message
	if ( result > 0 )
		return +1;
	
	// bail if we got an error
	if ( result < 0 )
		{
		ShowMessage( "* Log on read failed %d.", (int) result );
		return result;
		}
	
	// okay we got a message from the server
	// is it a log on message?
	result = BigToNativeEndian( response.logMsgTag );
	if ( result != kMsgLogOn )
		{
		// not a log on response
		// we are confused
		// try again
		ShowMessage( "* Expecting LogOn message from host got %d", (int) result );
		return +1;
		}
	
	// check the log on result
	result = BigToNativeEndian( response.logResult );
	switch ( result )
		{
		// success!
		case noErr:
			break;
		
		// some errors we can recover from
		case kBadCharName:
		case kBadCharPass:
			ShowMsgDlg( response.logData );
			*flags |= ( kLogOnGetName | kLogOnResetTimeout | kLogOnSend );
			result = +1;
			break;
		
		// errors we cannot recover from
		case kIncompatibleVersions:
			{
			ShowMsgDlg( response.logData );
			
			char buff[ 256 ];
			snprintf( buff, sizeof buff,
				_(TXTCL_UPDATE_FROM_WEB),
				kClanLordURL );
			ShowInfoText( buff );
			
			result = -1;
			}
			break;
		
		// special case if we need to download a new version of the client
		case kDownloadNewVersionLive:
		case kDownloadNewVersionTest:
			result = AutoUpdate( &response );
			if ( result < 0
			&&   result != -128
			&&	 result != appVersionTooOld
			&&	 result != kDownloadNewVersionLive )
				{
				GenericError( "%s", response.logData );
				}
			break;
		
		default:
			ShowMsgDlg( response.logData );
			result = -1;
		}
	
	return result;
}


/*
**	Handle1Comm()
**
**	handle one communication from the server
*/
DTSError
Handle1Comm()
{
	DTSError result;
	
	// read the newest message into our buffer
	DataSpool * spool = gNewMsgSpool;
	void * data = spool->GetData();
	size_t length = sizeof(Message);
	
	// if in movie mode, read from file instead
	if ( CCLMovie::IsReading() )
		{
		if ( CCLMovie::HasFrame() )
			{
			uint flags = 0;
			result = CCLMovie::ReadOneFrame( data, length, flags );
			
			// to be sure the packets are redirected properly to player window
			gPlayersListIsStale = bool(flags & CCLMovie::flag_Stale);
			}
		else
			result = +1;	// not the proper time for next frame
		}
	else
		result = gNetChannel.Read( data, &length );
	
	// if recording, go write the frame
	if ( CCLMovie::IsRecording() && length && noErr == result)
		{
		uint flags = 0;
		if ( gPlayersListIsStale )
			flags |= CCLMovie::flag_Stale;
		DTSError err = CCLMovie::WriteOneFrame( data, length, flags );
		if ( err )
			result = err;
		}
	
	spool->SetLimit( length );
	
	// bail if there was an error
	if ( result < noErr )
		{
		ShowMessage( "* Error reading from network channel %d.", (int) result );
		
		// just bail if we are viewing a movie
		if ( CCLMovie::IsReading() )
			{
			ShowDisconMsgs();
			gPlayingGame = false;
			}
		else
		if ( not gNetChannel.IsConnected() )	// punt if we got disconnected
			{
			ShowDisconMsgs();
			gPlayingGame = false;
			}
		
		return result;
		}
	
	// do nothing if no new data has come in.
	if ( result > noErr )
		return result;
	
	// handle the communication
	spool->SetMark( 0 );
	int tag = spool->GetNumber( kSpoolUnsignedShort );
	result = spool->GetResult();
	if ( noErr == result )
		{
		switch ( tag )
			{
			case kMsgDrawState:
				HandleDrawStateMsg();
				break;
			
#if 0
			// server has never sent these
			case kMsgPassword:
				{
				/* int error = */ spool->GetNumber( kSpoolSignedShort );
				char message[ 256 ];
				spool->GetString( message, sizeof message );
				ShowMsgDlg( message );
				} break;
#endif  // 0
			
			default:
				ShowMessage( "* Received an unexpected message: %d.", tag );
				break;
			}
		}
	
	return result;
}


/*
**	HandleDrawStateMsg()
**
**	we just got a draw state message from the server
*/
void
HandleDrawStateMsg()
{
	// swap the message buffers
	if ( gNewMsgSpool == &gMsgSpoolA )
		{
		gNewMsgSpool = &gMsgSpoolB;
		gDSSpool     = &gMsgSpoolA;
		}
	else
		{
		gNewMsgSpool = &gMsgSpoolA;
		gDSSpool     = &gMsgSpoolB;
		}
	
	// extract the important data from the message
	ExtractImportantData();
}


/*
**	SendInput()
**
**	send the user input to the server
*/
void
SendInput()
{
	// show the frame rate in milliseconds
	// cause no one understands ticks
	if ( gPrefsData.pdShowFrameTime )
		{
		static ulong lastTime = 0;
		ulong thisTime = GetFrameCounter();
		ShowMessage( "Milliseconds since last message: %d",
			int( thisTime - lastTime ) * 50 / 3 );
		lastTime = thisTime;
		}
	
	// get the input from the player
	PlayerInput playerInput;
	GetPlayerInput( &playerInput );
	
	// Determine which channel to send it on:
	// UDP most of the time, but TCP once every so often to keep
	// NAT boxes and other intermediaries from killing the reliable channel
	// due to inactivity
	static ulong nextReliableTime;
	int channel = kNetWriteUnreliable;
	
	// I arbitrarily picked a keep-alive interval of 3 minutes + random(120) seconds
	// if anyone has a better suggestion, please chime in.
	const ulong kKeepAliveInterval	= 3 * 60 * 60;
	const ulong kKeepAliveJitter	= 2 * 60 * 60;
	
	if ( GetFrameCounter() > nextReliableTime
	&&	 playerInput.piKeyString[ 0 ] == 0 )
		{
		channel = kNetWriteReliable;
		// next tcp packet will be 3 to 5 minutes from now
		nextReliableTime = GetFrameCounter() + GetRandom( kKeepAliveJitter ) + kKeepAliveInterval;
		
//		ShowMessage( "* KeepAlive packet at ack %d", (int) gAckFrame );
		}
	
	// send it to the server input
	size_t size = offsetof( PlayerInput, piKeyString ) + strlen(playerInput.piKeyString) + 1;
	DTSError result = gNetChannel.Write( &playerInput, size, channel );
	
	// bail if we got an error
	if ( result != noErr )
		{
		ShowMessage( "* Error writing to network channel %d.", (int) result );
		
		// punt if we got disconnected
		if ( not gNetChannel.IsConnected() )
			{
			ShowDisconMsgs();
			gPlayingGame = false;
			}
		}
}


/*
**	ShowDisconMsgs()
**
**	show disconnect messages to the user
*/
void
ShowDisconMsgs()
{
	ShowMessage( _(TXTCL_DISCONNECTED) );
}



/*
**	SendWaitReply()
**
**	wait for the message to be returned
*/
DTSError
SendWaitReply( Message * msg, size_t size, char * error )
{
	int tag = BigToNativeEndian( msg->msgTag );
	
	DTSError result = gNetChannel.Write( msg, size, kNetWriteReliable );
	if ( noErr == result )
		{
		ulong timeout = GetFrameCounter() + kLogOnTimeout;
		
		for (;;)
			{
			// run the application
			RunApp();
			
			// check if we quit
			if ( gDoneFlag )
				{
				result = 1;
				break;
				}
			
			// check for a reply
			size_t ssize = sizeof *msg;
			result = gNetChannel.Read( msg, &ssize );
			
			// stop if we failed
			if ( result < 0 )
				break;
			
			// stop if we succeeded
			// and the tag is correct
			if ( noErr == result
			&&   BigToNativeEndian( msg->msgTag ) == tag )
				{
				break;
				}
			
			// check for timeout
			ulong curtime = GetFrameCounter();
			if ( curtime > timeout )
				{
				result = -1;
				snprintf( error, 64, "Waiting for reply timed out after %d seconds.",
					kLogOnTimeout / 60 );
				break;
				}
			}
		}
	
	// special case download replies
	DTSError logResult = BigToNativeEndian( msg->msgLogOn.logResult );
	if ( noErr == result
	&& ( kDownloadNewVersionLive == logResult
	||   kDownloadNewVersionTest == logResult ) )
		{
		if ( kMsgLogOn == tag
		||   kMsgCharList == tag
		||   kMsgNewChar == tag
		||   kMsgDeleteChar == tag
		||   kMsgNewPassword == tag
		||   kMsgChallenge == tag
		||   kMsgIdentifiers == tag )
			{
			result = AutoUpdate( &msg->msgLogOn );
			}
		}
	
	if ( result < noErr
	&&   *error == 0 )
		{
		snprintf( error, 64, "Failed to send a request to the host. Error %d.", (int) result );
		}
	
	return result;
}


/*
**	CLNetChannel::ConnectCallBack()
**
**	connect host call back routine
*/
DTSError
CLNetChannel::ConnectCallBack( const char * status )
{
#ifdef DEBUG_VERSION
	static const char * lastmessage;
	if ( status
	&&   status != lastmessage )
		{
//		ShowMessage( "%s", status );
		}
	lastmessage = status;
#else
	#pragma unused( status )
#endif
	
	ulong posttime = gCallBackTime;
	ulong curtime  = GetFrameCounter();
	if ( curtime > posttime )
		{
		ShowInfoText( _(TXTCL_LOOKING_FOR_SERVER) );
		gCallBackTime = curtime + kCallBackDelay;
		}
	
	RunApp();
	if ( not gPlayingGame
	||   gDoneFlag )
		{
		return +1;
		}
	
	return noErr;
}


/*
**	AnswerChallenge()
**
**	prove we know the password by...
**	decoding the (random) challenge string, using the password as the key...
**	applying a one-way hash...
**	re-encoding it.
**	The server does the same on its end. If both sides produce the same final hash value,
**	the proof is complete. Moreover, the password is never transmitted in the clear.
*/
char *
AnswerChallenge( char * dst, const char * password, const char * challenge )
{
	// decode the challenge string
	size_t pwlen = strlen( password );
	char plaintext[ kChallengeLen ];
	DTSDecode( challenge, plaintext, kChallengeLen, password, pwlen );
	
	// hash it
	uchar hash[ 16 ];
	DTSOneWayHash( plaintext, kChallengeLen, hash );
	
	// encode it
	uchar decoded[ 16 ];
	DTSEncode( hash, decoded, sizeof hash, password, pwlen );
	memcpy( dst, decoded, sizeof hash );
	dst += sizeof hash;
	
	return dst;
}


#pragma mark -

/*
**	SendIdentifiers()
**
**	send our more/less unique identifiers to the server.
**	please do not distribute this information.
**	although not exactly *secret*
**	we really don't want the unique identifiers to be easily
**	obtained by snerts
**	ie those that would attempt to fake your identity and
**	cause grief to you and your friends.
**	thanks. -timmer dts
*/
DTSError
SendIdentifiers()
{
	LogOn logon;
#ifdef DEBUG_VERSION
//	memset( &logon, 0xFF, sizeof logon );
#endif
	SetupLogonRequest( &logon, kMsgIdentifiers );
	char * ptr = logon.logData;
	const char * limit = ptr + sizeof logon.logData;
	
	// append the file ID and contents of the invisible file
	// stored in the prefs folder
	ptr = GetMagicFileInfo( ptr );					// 8 bytes
	
	// append the ethernet hardware address
	ptr = GetEthernetAddress( ptr );				// 6 bytes
	
	// append the user name
	ptr = GetUserName( ptr, limit - ptr );			// variable length
	
	// and the machine name
	ptr = GetMachineName( ptr, limit - ptr );		// ditto
	
	// append the name of the boot volume
	ptr = GetBootVolumeName( ptr, limit - ptr );	// ditto
	
	// append what language we like
	// it's important that this come after any machine identification data
	ptr = GetRealLanguage( ptr, limit - ptr );		// 1 byte
	
	// hide it from snoops
	size_t sz = ptr - logon.logData;
	SimpleEncrypt( logon.logData, sz );
	
	// send it
	sz = ptr - reinterpret_cast<const char *>( &logon );
	DTSError result = gNetChannel.Write( &logon, sz, kNetWriteReliable );
	
	return result;
}


/*
**	GetRealLanguage()
**
**	append the user's preferred real language as a single byte
*/
char *
GetRealLanguage( char * ptr, size_t sz )
{
#ifdef MULTILINGUAL
	char lang = static_cast<char>( gRealLangID );
#else
	const char lang = static_cast<char>( kRealLangDefault );
#endif
	
	if ( sz >= sizeof lang )
		*ptr++ = lang;
	
	return ptr;
}


// name of the semi-hidden ID cache file
#define CL_ID_FNAME		"Clan Lord ID"

// this stupidity brought to you courtesy of the Xcode 3.2 debugger, which tends to go bonkers
// whenever it tries to display the value of an FSRef object. Worse, it often takes 30-60 seconds
// before it even _fails_ to display that value, which makes single-stepping an agony.
// By wrapping "naked" FSRefs inside this trivial modesty cloak, we can avoid such problems.
struct RefRap
{
	FSRef	r;
	
	// for convenience:
	operator const FSRef*() const	{ return &r; }
	operator FSRef*()				{ return &r; }
};


/*
**	GetMagicFileInfo()
**
**	create an invisible file in the prefs folder
**	write the random seed to it
**	give it a file ID
**	copy 8 bytes: 4 bytes each for seed & ID
**	
**	TODO:
**	- consider whether we still actually need this exact mechanism. It's awfully dependent
**	on deprecated APIs
*/
char *
GetMagicFileInfo( char * ptr )
{
	DTSError result;
	SInt32 fileID = 0;
	SInt32 randID = 0;
	
	// find/create the user's preferences folder
	RefRap fsr, prefRef;
//	if ( noErr == result )
		{
		result = FSFindFolder( kUserDomain, kPreferencesFolderType, kCreateFolder, prefRef );
		__Check_noErr( result );
		}
	
	// create the file
	FSCatalogInfo finfo;
	memset( &finfo, 0, sizeof finfo );
	if ( noErr == result )
		{
		// cheapo char-to-Unicode conversion; valid only because the name is pure 7-bit ascii
		HFSUniStr255 fname;
		UniChar * fnamep = fname.unicode;
		for ( const char * pp = CL_ID_FNAME; *pp; ++pp )
			*fnamep++ = UniChar( *pp );
		fname.length = fnamep - fname.unicode;
		
		// set the invisible bit
		// maybe the filename should have a leading '.' instead? or use chflags( UF_HIDDEN )?
		FileInfo& fileinfo = * reinterpret_cast<FileInfo *>( &finfo.finderInfo );
		fileinfo.finderFlags = kIsInvisible;	// | kNameLocked ??
		
		result = FSCreateFileUnicode( prefRef, fname.length, fname.unicode,
			kFSCatInfoFinderInfo, &finfo, fsr, nullptr );
		
		// not an error if it already existed from a previous run
		if ( dupFNErr == result )
			{
			// ... but in that case we need to get an FSRef for it
			result = FSMakeFSRefUnicode( prefRef, fname.length, fname.unicode,
						kTextEncodingUnknown, fsr );
			__Check_noErr( result );
			}
		}
	
	// get its fileID (aka nodeID)
	if ( noErr == result )
		{
		result = FSGetCatalogInfo( fsr, kFSCatInfoNodeID, &finfo, nullptr, nullptr, nullptr );
		__Check_noErr( result );
		fileID = finfo.nodeID;
		}
	
	// read the random ID, or (if that fails) regenerate it
	if ( noErr == result )
		{
		// open the file's data fork for reading
		SInt16 fnum = -1;
		result = FSOpenFork( fsr, 0, nullptr, fsRdPerm, &fnum );
		__Check_noErr( result );
		if ( noErr == result )
			{
			// read the 32-bit random ID
			ByteCount nRead;
			result = FSReadFork( fnum, fsFromStart, 0, sizeof randID, &randID, &nRead );
			__Verify_noErr( FSCloseFork( fnum ) );
			
			// if we failed to read the file
			// then pretend we couldn't open it
			if ( noErr != result || nRead != sizeof randID )
				fnum = -1;
			}
		
		// we could not open the file (or read the complete random ID)
		// recreate its contents with a new ID and secret message
		if ( -1 == fnum )
			{
			// we know the file exists, cos we created it up above
			// open data fork for writing
			result = FSOpenFork( fsr, 0, nullptr, fsWrPerm, &fnum );
			
			if ( noErr == result )
				{
				// generate a new random ID, and write it out
				randID = GetRandom( UINT_MAX );
				ByteCount nWrote;
				result = FSWriteFork( fnum, fsFromStart, 0, sizeof randID, &randID, &nWrote );
				__Check_noErr( result );
				
				// write the static data
				// curiosity kills cats -timmer
				char buff[] =
					"\x36\x50\x2A\xFC\xCB\xA1\x4E\x3B\x1D\xE6\xC9\xA7\x48\x33\x06\xFD"
					"\xD6\xE7\x1C\x03\x06\xE6\x85\xA0\x53\x2F\x07\xF7\x85\xA7\x1C\x29"
					"\x0C\xF0\xD7\xA3\x48\x7A\x04\xF6\xD6\xB5\x5D\x3D\x0C\xB2\xAF\x80"
					"\x53\x28\x49\xFA\xCB\xA0\x53\x28\x04\xF2\xD1\xAF\x53\x34\x49\xF2"
					"\xC7\xA9\x49\x2E\x49\xE7\xCD\xA3\x1C\x2A\x1C\xE1\xD5\xA9\x4F\x3F"
					"\x49\xFC\xC3\xE6\x48\x32\x00\xE0\x85\xA0\x55\x36\x0C\xBD\x8B\xE8"
					"\x36\x3B\x07\xF7\x85\xB2\x53\x7A\x19\xE1\xCA\xB0\x59\x7A\x1D\xFB"
					"\xC4\xB2\x1C\x2E\x01\xF6\x85\xA8\x55\x39\x0C\xB3\xC3\xA9\x50\x31"
					"\x1A\xB3\xC4\xB2\x1C\x1E\x0C\xFF\xD1\xA7\x1C\x0E\x08\xFC\x85\xA5"
					"\x4E\x3F\x08\xE7\xC0\xA2\x1C\x33\x1D\xBD\x8B\xE8\x36\x3B\x07\xF7"
					"\x85\xB1\x54\x23\x49\xEA\xCA\xB3\x1C\x29\x01\xFC\xD0\xAA\x58\x7A"
					"\x07\xFC\xD1\xE6\x58\x3F\x05\xF6\xD1\xA3\x1C\x33\x1D\xBD\x8B\xE8"
					"\x36\x34\x06\xE1\x85\xA3\x44\x2A\x06\xE0\xC0\xE6\x55\x2E\x47\xBD"
					"\x8B\xCC\x5B\x35\x49\xE7\xCA\xE6\x00\x32\x1D\xE7\xD5\xFC\x13\x75"
					"\x1E\xE4\xD2\xE8\x58\x3F\x05\xE7\xC4\xB2\x5D\x35\x47\xF0\xCA\xAB"
					"\x13\x29\x0C\xF0\xD7\xA3\x48\x37\x0C\xE0\xD6\xA7\x5B\x3F\x46\xF0"
					"\xC9\xA7\x52\x36\x06\xE1\xC1\xAF\x58\x74\x01\xE7\xC8\xAA\x02\x50"
					"\x1C\xE0\xC0\xB4\x06\x7A\x0A\xE6\xD7\xAF\x53\x29\x00\xE7\xDC\xCC"
					"\x4C\x3B\x1A\xE0\xD2\xA9\x4E\x3E\x53\xB3\xCE\xAF\x50\x36\x1A\xF0"
					"\xC4\xB2\x4F\x50\x44\xE7\xCC\xAB\x51\x3F\x1B\x99";
				
				enum { buflen = sizeof buff - 1 };	// ignoring final NUL
				SimpleEncrypt( buff, buflen );
				if ( noErr == result )
					result = FSWriteFork( fnum, fsAtMark, 0, buflen, buff, &nWrote );
				
				memset( buff, 0, buflen );	// help prevent caticide
				
				// close the file
				__Verify_noErr( FSCloseFork( fnum ) );
				if ( noErr != result || nWrote != buflen )
					result = -2;	// failed to write complete message
				}
			else	// failed to open fork for writing
				result = -3;
			}
		}
	
	// copy the file-ID and the rand ID
	fileID = NativeToBigEndian( fileID );
	memcpy( ptr, &fileID, sizeof fileID );
	memcpy( ptr + sizeof fileID, &randID, sizeof randID );
	ptr += sizeof fileID + sizeof randID;
	
	return ptr;
}


/*
**	GetEthernetAddress()
**
**	return the Ethernet MAC address of this computer
**	(or one of 'em, anyway)
**	6 bytes
*/
char *
GetEthernetAddress( char * ptr )
{
	enum { kEnetMACAddrLen = 6U };
	DTSError result = DTSEthernetHardwareAddress( ptr );
	
	// copy the hardware address or use zeros
	if ( result != noErr )
		memset( ptr, 0, kEnetMACAddrLen );
	
	return ptr + kEnetMACAddrLen;
}


/*
**	GetUserName()
**
**	append the (nul-terminated) user name string to 'ptr'
*/
char *
GetUserName( char * ptr, size_t sz )
{
	if ( CFStringRef name = CSCopyUserName( false ) )	// false -> full user name
		{
		// first try to get it in MacRoman encoding
		bool good = CFStringGetCString( name, ptr, sz, kCFStringEncodingMacRoman );
		if ( not good )
			{
			// if that fails, try to get it in UTF8
			good = CFStringGetCString( name, ptr, sz, kCFStringEncodingUTF8 );
			}
		if ( not good )
			{
			// if THAT fails then the buffer must be too small (right?)
			strlcpy( ptr, "<long>", sz );
			}
		CFRelease( name );
		}
	else
		strlcpy( ptr, "<none>", sz );	// failsafe for CSCopyUserName()
	
	return ptr + strlen( ptr ) + 1;
}


/*
**	GetMachineName()
**
**	The current format is:
**		<arch>/X.Y.Z: <machine_name>
**	where <arch> is "MachO_x86", and X.Y.Z is the current OS version number
**	(we used to have a lot more architectural variety, in earlier times)
*/
char *
GetMachineName( char * ptr, size_t sz )
{
	// fetch machine name as a C string (see GetUserName() for encoding strategy)
	char tmp[ 256 ];
	if ( CFStringRef name = CSCopyMachineName() )
		{
		bool good = CFStringGetCString( name, tmp, sizeof tmp, kCFStringEncodingMacRoman );
		if ( not good )
			good = CFStringGetCString( name, tmp, sizeof tmp, kCFStringEncodingUTF8 );
		if ( not good )
			strcpy( tmp, "<long>" );
		CFRelease( name );
		}
	else
		strcpy( tmp, "<none>" );
	
#if TARGET_CPU_PPC
# define CLIENT_TYPE "MachO_PPC"
#elif TARGET_CPU_X86
# define CLIENT_TYPE "MachO_x86"
#else
# error "unknown CPU"
#endif  // TARGET_CPU_xxx

	// combine client type, OS version, and machine name into 'ptr'
	snprintf( ptr, sz, CLIENT_TYPE "/%d.%d.%d: %s",
		gOSFeatures.osVersionMajor,
		gOSFeatures.osVersionMinor,
		static_cast<int>( gOSFeatures.osVersionBugFix ),
		tmp );
	return ptr + strlen( ptr ) + 1;
	
#undef CLIENT_TYPE
}


/*
**	GetBootVolumeName()
**
**	return the name of the boot volume
*/
char *
GetBootVolumeName( char * ptr, size_t sz )
{
	// got room, for trailing null at least?
	if ( sz < 1 )
		return ptr;
	
#if 1  // the newer (but by no means modern) way
	
	// start the search from a location expected to be _somewhere_ within the root volume
	RefRap ref;
	FSCatalogInfo info;
	HFSUniStr255 vName;
	OSStatus result = FSFindFolder( kSystemDomain,
						kVolumeRootFolderType,	// kDomainTopLevelFolderType,
						kCreateFolder, ref );
	__Check_noErr( result );
	info.parentDirID = 0;	// any value that's != to fsRtParID
	
	// then ascend to root directory of volume
	while ( noErr == result && fsRtParID != info.parentDirID )
		{
		result = FSGetCatalogInfo( ref, kFSCatInfoParentDirID, &info, &vName, nullptr, ref );
		__Check_noErr( result );
		}
	
	// get its name
	CFStringRef volName = nullptr;
	if ( noErr == result )
		volName = FSCreateStringFromHFSUniStr( kCFAllocatorDefault, &vName );
	else
		{
		// had error in FSGetCatInfo or FSFindFolder() -- return something innocuous
		volName = CFSTR("/");
		CFRetain( volName );	// for the looming CFRelease below
		}
	
	if ( volName )
		{
		// copy name to 'ptr' -- ideally as MacRoman, but fall back to UTF8 if need be
		bool got = CFStringGetCString( volName, ptr, sz, kCFStringEncodingMacRoman );
		if ( not got )
			got = CFStringGetCString( volName, ptr, sz, kCFStringEncodingUTF8 );
		if ( not got )
			strlcpy( ptr, "<bad>", sz );
	
		CFRelease( volName );
		}
	else
		strlcpy( ptr, "<err>", sz );	// FSCreateStringFromHFSUniStr() failed !?!?
	
	ptr += strlen( ptr ) + 1;
	
#else  // the older way
	Str255 name;
	CInfoPBRec pb;
	
	// find the system folder's vRefNum
	SInt32 sysDirID;
	SInt16 sysVRefNum;
	DTSError result = FindFolder( kOnSystemDisk, kSystemFolderType,
								kDontCreateFolder, &sysVRefNum, &sysDirID );
	
	// fill out the paramblock
	pb.dirInfo.ioCompletion = nullptr;
	pb.dirInfo.ioNamePtr    = name;
	
	// use the vRefNum from FindFolder if it succeeded, else use the 1st volume
	if ( noErr == result )
		pb.dirInfo.ioVRefNum = sysVRefNum;
	else
		pb.dirInfo.ioVRefNum = -1;
	
	pb.dirInfo.ioFDirIndex  = -1;
	pb.dirInfo.ioDrDirID    = fsRtDirID;
	
	result = PBGetCatInfoSync( &pb );
	
	if ( noErr == result )
		{
		uint len = name[0];
		if ( len >= sz )
			len = sz - 1;
		memcpy( ptr, name + 1, len );
		ptr += len;
		}
	
	// null terminate
	*ptr++ = 0;
#endif  // 1
	
	return ptr;
}


