/*
**	ProgressWin_cl.h		ClanLord
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

#ifndef PROGRESSWIN_CL_H
#define	PROGRESSWIN_CL_H


/*
**	class ProgressWin
**	minor little free-floating progress bar
**	(cannibalized from editor source)
*/
class ProgressWin : public DTSWindow
{
protected:
	HIViewRef		progressCtl;	// progress bar
	long			prevSleep;		// saved DTSApp sleep
	CFAbsoluteTime	creationTime;	// to prevent flicker
	
	// all these only apply to the downloading variant
	HIViewRef		label1;			// static-text ctl, displays "X of Y (Z KBps)"
	HIViewRef		label2;			// displays estimated time remaining
//	bool			mIndeterminate;	// true if progress bar in non-determinate state
	
	static	ProgressWin * pwin;		// there's only ever one
	
	virtual void	Show();
	static OSStatus	InstallControls();
	
	void			UpdateDLStats( ulong cur, ulong lim ) const;	// DL helper
	CFTimeInterval	Age() const { return CFAbsoluteTimeGetCurrent() - creationTime; }
	
		// private because you can't create instances;
		// all access via static functions
	ProgressWin() :
			progressCtl( nullptr )
			, prevSleep( 0 )
			, creationTime( CFAbsoluteTimeGetCurrent() )
			, label1( nullptr )
			, label2( nullptr )
//			, mIndeterminate( false )
		{
		}
	
public:
		// the public interface is very simple: 
		//	call Setup("title") to make the window...
		//	periodically call SetProgress( x, y ), where you've completed
		//		'x' out of 'y' "work units", whatever those are...
		//	call Dispose() when you've finished.
		//
		// For the download-progress variant, use SetupDL( dlFileName ).
		// SetProgress( 0, 0 ) is a special case, and makes it "indeterminate" 
		
	static void		SetProgress( ulong curr, ulong limit );
	static void		Setup( const char * title );
	static void		SetupDL( CFStringRef filename );
	static void		Dispose();
};


/*
**	class StProgressWindow
**	an even simpler stack-based wrapper for the above.
**	You just write:
**		StProgressWindow progWin( "title" );
**		while ( ... ) {
**			progWin.SetProgress( x, y );
**			<real code here>
**		}
**	and it automatically vanishes when it goes out of scope.
*/
class StProgressWindow
{
public:
	StProgressWindow( const char * title )			{ ProgressWin::Setup( title ); }
	StProgressWindow( const HFSUniStr255& fname );	// for downloading
	~StProgressWindow()								{ ProgressWin::Dispose(); }
	void SetProgress( ulong cur, ulong lim ) const	{ ProgressWin::SetProgress( cur, lim ); }
	
private:
	StProgressWindow();								// not implemented: can't create w/o title
	StProgressWindow( const StProgressWindow& );	// copy ctor, not impl.
	StProgressWindow& operator=( const StProgressWindow& );		// operator=(), ditto
};

#endif	// PROGRESSWIN_CL_H
