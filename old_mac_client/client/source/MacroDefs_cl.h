/*
**	MacroDefs_cl.h		ClanLord
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

#ifndef	MACRODEFS_CL_H
#define MACRODEFS_CL_H

#include "Public_cl.h"
#include "Macros_cl.h"


#define kCLMacros_FolderName				"Macros"
#define kCLMacros_DefaultFileName			"Default"
#define kCLMacros_InstructionFileName		"Macro Instructions"

#define kCLMacros_BreakString				" \t\r\n"

const int	kCLMacros_MaxCmdParam		 	= 10;

typedef	SafeString			MacroParamRec[ kCLMacros_MaxCmdParam ];
typedef int					MacroCmdSet[];


//
// Class for mapping macro IDs, command strings, and special variables
//
struct CMacroKind
{
	const char *	mStr;
	int				mID;
	
	static int			MacroNameToID( const CMacroKind * const array,
									const char * name, const char * breaks = "" );
	static int			MacroNameBeginningToID( const CMacroKind * const array,
									const char * str );
	static void			MacroIDToName( const CMacroKind * const array,
									int m_id, SafeString * dest );
	
	
	enum { vEnv = 1, vMy, vSelPlayer, vClick, vRandom };
	enum {	// vEnv
		vKeyInterrupts = 1,
		vClickInterrupts,
		vDebug,
		vEcho,
		vUnfriendly,
		vTextLog
		};
	
	enum { 	// vMy and vSelPlayer
		vRealName = 1,
		vSimpleName,
		vLeftItem,
		vRightItem,
		vRace,
		vGender,
		vHealth,
		vBalance,
		vMagic,
		
		vSharesIn,
		vSharesOut,
		
		// from vForehead through vHead must remain contiguous and in this order
		vForehead,
		vNeck,
		vShoulders,
		vArms,
		vGloves,
		vFinger,
		vCoat,
		vCloak,
		vTorso,
		vWaist,
		vLegs,
		vFeet,
		vBothHands,
		vHead,
		
		vSelectedItem
		
		}; // vMy and vSelPlayer (not all are available for vSelPlayer)
	
	enum { vWord = 1, vLetter, vNumWords, vNumLetters }; // Any variable
	
	// Obsolete variables
	enum {  vName = 1, vSimplePlayerName, vplayerName, vRightHandName, vLeftHandName,
			vOldEcho, vOldDebug, vOldInterruptClick, vOldInterruptKey, vOldClickSPlayer,
			vOldClickRPlayer, vOldWordCount };
	
	enum
		{
		// Macros
		mEmpty = 0, mExpression, mReplacement, mFunction, mKey, mIncludeFile, mVariable,
		
		// Commands
		cPause, cMove,
		cSetVariable, cSetGlobalVariable, cCallFunction,
		cEnd, cIf, cElse, cElseIf, cEndIf,
		cRandom, cOr, cEndRandom, cLabel, cGoto,
		cText, cMessage, cParameter, cNotCaseSensitive,
		cStart, cFinish
		};
	
	// Attributes
	enum
		{
		aIgnoreCase 		= 0x00000001,
		aAnyClick		 	= 0x00000002,
		aNoOverride		 	= 0x00000008
		};
};


//
// Base class for macros
//
// A linked list
//
class CMacro : public DTSLinkedList<CMacro>
{
public:
	int					mKind;
	
	static CMacro *		sRootMacro;		// base of linked list of all macros
	
							CMacro( int kind, CMacro ** root );
	
	static void 			DeleteAll( CMacro ** root );
	static CMacro * 		FindMacro( int kind, CMacro * root );
	
	static void 			VShowMacroInfoText( bool mustShow,
									CMacro * root,
									int currentLine,
									const char * fileName,
									const char * format,
									std::va_list params );
	
	static void				ShowMacroInfoText( bool mustShow,
								   CMacro * root,
								   int currentLine,
								   const char * fileName,
								   const char * format, ... ) PRINTF_LIKE( 5, 6 );
	
	static void				ShowMacroInfoText( const char * format, ... ) PRINTF_LIKE( 1, 2 );
	static void				SetTextWinBuffer( const char * text );
	
	// storage for the most recent line sent to text window, for @env.textLog
	static char		sTextWinLine[ kPlayerInputStrLen ];
	
private:
	// no copying or default construction
							CMacro();
							CMacro( const CMacro& );
	CMacro&					operator=( const CMacro& );
};


//
// Macro Types
//

class CTriggerMacro : public CMacro
{
public:
	CMacro *			mTriggers;
	uint				mAttributes;	// ignore-case, etc.
	
							CTriggerMacro( int kind, CMacro ** root ) :
									CMacro( kind, root ),
									mTriggers( nullptr ),
									mAttributes( 0 )
									{}
	
private:
							CTriggerMacro();
							CTriggerMacro( const CTriggerMacro& );
	CTriggerMacro&			operator=( const CTriggerMacro& );
};


class CExpressionMacro : public CTriggerMacro
{
public:
	SafeString			mExpression;
	
							CExpressionMacro( const char * expression, CMacro ** root );
	static CExpressionMacro *	FindMacro( const char * expression, CMacro * root );

private:
							CExpressionMacro();
							CExpressionMacro( const CExpressionMacro& );
	CExpressionMacro&		operator=( const CExpressionMacro& );
};
	

class CFunctionMacro : public CTriggerMacro
{
public:
	SafeString			mName;
		
							CFunctionMacro( const char * name, CMacro ** root );
	static CFunctionMacro *		FindMacro( const char * name, CMacro * root );

private:
							CFunctionMacro();
							CFunctionMacro( const CFunctionMacro& );
	CFunctionMacro&			operator=( const CFunctionMacro& );
};
	
	
class CReplacementMacro : public CTriggerMacro
{
public:
	SafeString			mReplace;
		
							CReplacementMacro( const char * replace, CMacro ** root );
	static CReplacementMacro * 	FindMacro( const char * replace, CMacro * root );

private:
							CReplacementMacro();
							CReplacementMacro( const CReplacementMacro& );
	CReplacementMacro&		operator=( const CReplacementMacro& );
};


class CKeyMacro : public CTriggerMacro
{
public:
	int					mKey;			// keystroke that triggers us
	uint				mModifiers;		// ... if these modifiers match
	
							CKeyMacro( int key, uint modifiers, CMacro ** root ) :
								CTriggerMacro( CMacroKind::mKey, root ),
								mKey( key ),
								mModifiers( modifiers )
								{}
	static CKeyMacro * 		FindMacro( int key, uint modifiers, CMacro * root );

private:
							CKeyMacro();
							CKeyMacro( const CKeyMacro& );
	CKeyMacro&				operator=( const CKeyMacro& );
};


class CMarkMacro : public CTriggerMacro
{
public:
	CMacro *			mFirst;
		
	explicit				CMarkMacro( CMacro ** root ) :
								CTriggerMacro( CMacroKind::mFunction, root ),
								mFirst( nullptr )
								{}
private:
							CMarkMacro();
							CMarkMacro( const CMarkMacro& );
	CMarkMacro&				operator=( const CMarkMacro& );
};

	
class CIncludeFileMacro : public CMacro
{
public:
	SafeString			mFileName;
	
							CIncludeFileMacro( const char * name, CMacro ** root );
	static CIncludeFileMacro *	FindMacro( const char * name, CMacro * root );

private:
							CIncludeFileMacro();
							CIncludeFileMacro( const CIncludeFileMacro& );
	CIncludeFileMacro&		operator=( const CIncludeFileMacro& );
};


class CVariableMacro : public CMacro
{
public:
	SafeString			mValue;
	SafeString			mName;
	
							CVariableMacro( const char * name, CMacro ** root );
	
	static CVariableMacro * FindMacro( const char * name, CMacro * root );
	static void				ArrayElementToVarName( SafeString * name, CMacro * root );
	static void				SetVariable( const char * name, const char * value,
										 CMacro ** root, bool dontResolve = false );
	
	static bool				GetEnvVar( const char * name, SafeString * dst );
	static bool				GetMyVar( const char * name, SafeString * dst );
	static bool				GetSelPlayerVar( const char * name, SafeString * dst );
	static bool				GetVariable( const char * name, SafeString * dst, CMacro * root );
	
//	static bool				CompareVariable( const char * name, const char * val, CMacro * root );
	static bool				TestBool( const char * name, CMacro * root );

private:
							CVariableMacro();
							CVariableMacro( const CVariableMacro& );
	CVariableMacro&			operator=( const CVariableMacro& );
};


//
// Command Types
//

class CLabelCmdMacro : public CMacro
{
public:
	SafeString		mName;
	
							CLabelCmdMacro( const char * name, CMacro ** root );
	static CLabelCmdMacro *	FindMacro( const char * name, CMacro * root );

private:
							CLabelCmdMacro();
							CLabelCmdMacro( const CLabelCmdMacro& );
	CLabelCmdMacro&			operator=( const CLabelCmdMacro& );
};


class CRandomCmdMacro : public CMacro
{
public:
	int				mLastChosen;
	
	explicit				CRandomCmdMacro( CMacro ** root ) :
								CMacro( CMacroKind::cRandom, root ),
								mLastChosen( -1 )
								{}
private:
							CRandomCmdMacro();
							CRandomCmdMacro( const CRandomCmdMacro& );
	CRandomCmdMacro&		operator=( const CRandomCmdMacro& );
};


class CParameterCmdMacro : public CMacro
{
public:
	SafeString		mParam;
	
							CParameterCmdMacro( const char * param, CMacro ** root );
private:
							CParameterCmdMacro();
							CParameterCmdMacro( const CParameterCmdMacro& );
	CParameterCmdMacro&		operator=( const CParameterCmdMacro& );
};


//
// Parses a macro file into linked lists of commands
//
class CMacroParser
{
public:
	SafeString 			fileName;
	int 				cmdLevel;
	int					currentLine;
	CTriggerMacro * 	lastMacro;
	
							CMacroParser();
	DTSError				ParseFile( const char * fname );
	DTSError				ParseLine( const char * line );
	DTSError 				NewMacro( CTriggerMacro *& macro, char * word, const char *& line );
	DTSError				NewCommand( char * word, const char *& line );
	DTSError				NewParameter( char * word, const char *& line );
	const char * 			NewWord( const char * inLine, char * outDest );
	void					IgnoreComment( std::FILE * );
	
	static const char *		GetWord( const char * inLine,
									 char * outDest,
									 size_t inMaxLen = kPlayerInputStrLen,
									 const char * inQuotes = nullptr,
									 const char * inSeparators = nullptr,
									 int line = 0,
									 const char * file = nullptr );
protected:
	void					ParseMsg( bool mustShow, const char * fmt, ... ) PRINTF_LIKE( 3, 4 );

private:
							CMacroParser( const CMacroParser& );
	CMacroParser&			operator=( const CMacroParser& );
};


//
//	Linked list of macros which are executing
//
class CExecutingMacro : public DTSLinkedList<CExecutingMacro>
{
public:
	CMarkMacro * 	exMark;
	CMacro * 		exVars;
	SafeString		exBuffer;
	int 			exWaitUntil;
	int				exKind;
	bool			exDebug;
	bool			exUnfriendly;
	
	explicit			CExecutingMacro( CTriggerMacro * macro );
						~CExecutingMacro();
	
	DTSError			Continue();
	DTSError			GetParameters(	CMacro ** exMacro,
										MacroParamRec * ioParams,
										int& ioParamsCount );
	void				Pause( int frames );
	DTSError			ExecuteCommand( CMacro ** exMacro, CMacro * exStart );
	
	static bool			FindCmdOnSameLevel( CMacro ** start, int numCmds,
											const MacroCmdSet cmdSet );
	
	static bool		sExInterrupted;
	static uint		sNumGotos;
	
private:	// no copying
						CExecutingMacro();
						CExecutingMacro( const CExecutingMacro& );
	CExecutingMacro&	operator= ( const CExecutingMacro& );
};


#endif	// MACRODEFS_CL_H

