/*
**	RegExp_dts.h		dtslib2
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

/*
**	See RegExp_dts.cp for additional info.
*/

#ifndef RegExp_dts_h
#define RegExp_dts_h

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif


/*
 * Definitions etc. for regexp(3) routines.
 *
 * Caveat:  this is V8 regexp(3) [actually, a reimplementation thereof],
 * not the System V one.
 */


/*
[Note: enough people have made enough changes to this codebase over enough years
that the following summary is probably no longer 100% reliable. Use the source.]

In summary, regexps can be:

abcd -- matching a string literally
. -- match any character except NUL
^ -- beginning of a line
$ -- end of a line
[a-z_?], [^a-z_?], [[:alpha:]] and [^[:alpha:]] -- matching character sets
(subexp) -- grouping an expression into a subexpression.
\< -- matches at the beginning of a word
\> -- matches at the end of a word

The following special characters and sequences can be applied to a character, character set,
subexpression, or backreference:

* -- repeat the preceeding element 0 or more times.
+ -- repeat the preceeding element 1 or more times.
? -- match the preceeding element 0 or 1 time.
{m,n} -- match the preceeding element at least m, and as many as n times.
regexp-1|regexp-2|.. -- match any regexp-n.
A special character, like . or * can be made into a literal character by prefixing it with \.
A special sequence, like \< or \> can be made into a literal character by dropping the \.
*/


#undef DEBUG_REGEXP
// #define DEBUG_REGEXP 1



class CHSRegExp
{
public:
	// maximum permissible number of parenthesized subexpressions
	static const int NSUBEXP   = 10;
	
	// The zeroeth element of each array is the *entire* matched regular expression
	const char * mStartPtrs[ NSUBEXP ];	// Pointers to start of parenthesized sub-re's
	const char * mEndPtrs[ NSUBEXP ];	// Pointers to end of parenthesized sub-re's
	
	typedef	void (*tRegErrorProc)( const char * errorString );
	
	explicit	CHSRegExp( const char * theRegExp, tRegErrorProc proc = nullptr );
	virtual		~CHSRegExp();
	bool	 	regexec( const char * theData );
	void		setErrorProc( tRegErrorProc theErrorProc ) { mRegErrorProc = theErrorProc; }
	
private:
	static const uchar MAGIC	= 0234;
	
	// disable copying
				CHSRegExp( const CHSRegExp& );
	CHSRegExp&	operator=( const CHSRegExp& );
	
	char *		reg( bool paren /* Parenthesized? */, int * flagp );
	char *		regbranch( int * flagp );
	char *		regpiece( int * flagp );
	char *		regatom( int * flagp );
	char *		regnode( char op );
	char *		regnext( char * p ) const;
	void 		regc( char c );
	void 		reginsert( char op, char * opnd );
	void 		regtail( char * p, const char * val );
	void 		regoptail( char * p, const char * val );
	bool		regtry( const char * string );
	bool		regmatch( char * prog );
	int			regrepeat( const char * p );
	void		regError( const char * errorString ) const DOES_NOT_RETURN;
	
	tRegErrorProc	mRegErrorProc;
	
	char			mRegstart;		/* Internal use only. */
	bool			mReganch;		/* Internal use only. */
	const char *	mRegmust;		/* Internal use only. */
	char *			mProgram;
	
	/*
	 * Global work variables for regcomp()
	 */
	const char *	mRegparse;		/* Input-scan pointer. */
	int 			mRegnpar;		/* () count. */
	char		 	mRegdummy;
	char *			mRegcode;		/* Code-emit pointer; &mRegdummy = don't. */
	int				mRegsize;		/* Code size. */
	
	/*
	 * Global work variables for regexec().
	 */
	const char *	mReginput;		/* String-input pointer. */
	const char *	mRegbol;		/* Beginning of input, for ^ check. */
	const char **	mRegstartp;		/* Pointer to startp array. */
	const char **	mRegendp;		/* Ditto for endp. */
	
#ifdef DEBUG_REGEXP
	bool			mRegnarrate;
	void			regdump() const;
	const char *	regprop( const char * op ) const;
#endif
};

#endif	// RegExp_dts_h

