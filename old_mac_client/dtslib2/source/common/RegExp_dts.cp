/*
**	RegExp_dts.cp		    dtslib2
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

/* Note: this file is formatted for 4 char tabs. */
/*
 *	Copyright (c) 1986 by University of Toronto.
 *	Written by Henry Spencer.  Not derived from licensed software.
 *
 *	Permission is granted to anyone to use this software for any
 *	purpose on any computer system, and to redistribute it freely,
 *	subject to the following restrictions:
 *
 *	1. The author is not responsible for the consequences of use of
 *		this software, no matter how awful, even if they arise
 *		from defects in it.
 *
 *	2. The origin of this software must not be misrepresented, either
 *		by explicit claim or by omission.
 *
 *	3. Altered versions must be plainly marked as such, and must not
 *		be misrepresented as being the original software.
 *** THIS IS AN ALTERED VERSION.  It was altered by John Gilmore,
 *** hoptoad!gnu, on 27 Dec 1986, to add \n as an alternative to |
 *** to assist in implementing egrep.
 *** THIS IS AN ALTERED VERSION.  It was altered by John Gilmore,
 *** hoptoad!gnu, on 27 Dec 1986, to add \< and \> for word-matching
 *** as in BSD grep and ex.
 *** THIS IS AN ALTERED VERSION.  It was altered by John Gilmore,
 *** hoptoad!gnu, on 28 Dec 1986, to optimize characters quoted with \.
 *** THIS IS AN ALTERED VERSION.  It was altered by James A. Woods,
 *** ames!jaw, on 19 June 1987, to quash a regcomp() redundancy.
 *** THIS IS AN ALTERED VERSION.  It was altered by Thomas R. Kimpton,
 *** tom@dtint.dtint.com, on 21Oct92, to use Macintosh handles, and
 *** explicit types, i.e. longs or shorts, no ints.
 *
 *** THIS IS AN ALTERED VERSION.  It was altered by Thomas R. Kimpton,
 *** tom@itsnet.com, on 29Aug96. This is now a PowerPlant object (I'm
 *** hoping it's also a C++ object :-). All the static 'helper' variables
 *** are now private member variables. And, because I wanted to see if I
 *** could make this code 'portable', I decided to use malloc/free instead
 *** of Macintosh Handles, and C strings.
 *
 *** THIS IS AN ALTERED VERSION. It was altered for Delta Tao Software
 *** during 2002-2007.
 *** This version adds const-correctness, modernizes the C++ idiom, and corrects comments.
 *** Also: Converted malloc()/free() to new/delete.
 *
 * Beware that some of this code is subtly aware of the way operator
 * precedence is structured in regular expressions.  Serious changes in
 * regular-expression syntax might require a total rethink.
 *
 * TODO: if TARGET_API_MAC_OSX, should we link to the standard library's
 * regexec(3) and friends, rather than this dodgy off-brand imitator?
 */

/*
[Note: enough people have made enough changes to this codebase over enough years
that the following summary is probably no longer 100% reliable. Use the source.]

In summary, regexps can be:

abcd -- match a string literally
. -- match any character except NUL
^ -- beginning of a line
$ -- end of a line
[a-z_?], [^a-z_?], [[:alpha:]] and [^[:alpha:]] -- matching character sets
(subexp) -- grouping an expression into a subexpression.
\< -- matches at the beginning of a word
\> -- matches at the end of a word

The following special characters can be applied to a character, character set,
or parenthesized subexpression:

* -- repeat the preceeding element 0 or more times.
+ -- repeat the preceeding element 1 or more times.
? -- match the preceeding element 0 or 1 time.
{m,n} -- match the preceeding element at least m, and as many as n times.
regexp-1\|regexp-2\|.. -- match any regexp-n.

A special character, like . or * can be made into a literal character by prefixing it with \.
A special sequence, like \< or \> can be made into a literal character by dropping the \.
*/

// TODO: 
//	1. The following locale-dependent <cstring> routines should be replaced
//	with our own MacRoman-aware / case-insensitive ones:
//
//	strstr()
//	strchr()
//	isalnum()
//	strncmp()
//
//	2. Additional Perl-style assertions:
//	\s \S \b \B \d \D \w \W


#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

// #include "RegExp_dts.h"

#ifndef __MACERRORS__
# define paramErr (-1)
#endif

/*
 * The "internal use only" fields in regexp.h are present to pass info from
 * compile to execute that permits the execute phase to run lots faster on
 * simple cases.  They are:
 *
 * mRegstart	char that must begin a match; '\0' if none obvious
 * mReganch		is the match anchored (at beginning-of-line only)?
 * mRegmust		string (pointer into mProgram) that match must include, or NULL
 *
 * mRegstart and mReganch permit very fast decisions on suitable starting points
 * for a match, cutting down the work a lot.  mRegmust permits fast rejection
 * of lines that cannot possibly match.  The mRegmust tests are costly enough
 * that regcomp() supplies a mRegmust only if the r.e. contains something
 * potentially expensive (at present, the only such thing detected is * or +
 * at the start of the r.e., which can involve a lot of backup).
 */

/*
 * Structure for regexp "program".  This is essentially a linear encoding
 * of a nondeterministic finite-state machine (aka syntax charts or
 * "railroad normal form" in parsing technology).  Each node is an opcode
 * plus a "next" pointer, possibly plus an operand.  "Next" pointers of
 * all nodes except BRANCH implement concatenation; a "next" pointer with
 * a BRANCH on both ends of it is connecting two alternatives.  (Here we
 * have one of the subtle syntax dependencies:  an individual BRANCH (as
 * opposed to a collection of them) is never concatenated with anything
 * because of operator precedence.)  The operand of some types of node is
 * a literal string; for others, it is a node leading into a sub-FSM.  In
 * particular, the operand of a BRANCH node is the first node of the branch.
 * (NB this is *not* a tree structure:  the tail of the branch connects
 * to the thing following the set of BRANCHes.)  The opcodes are:
 */

/* definition		number		opnd?	meaning */
#define	END			0		/* 	no		End of program. */
#define	BOL			1		/* 	no		Match "" at beginning of line. */
#define	EOL			2		/* 	no		Match "" at end of line. */
#define	ANY			3		/* 	no		Match any one character. */
#define	ANYOF		4		/* 	str		Match any character in this string. */
#define	ANYBUT		5		/* 	str		Match any character not in this string. */
#define	BRANCH		6		/* 	node	Match this alternative, or the next... */
#define	BACK		7		/* 	no		Match "", "next" ptr points backward. */
#define	EXACTLY		8		/* 	str		Match this string. */
#define	NOTHING		9		/*	no		Match empty string. */
#define	STAR		10		/* 	node	Match this (simple) thing 0 or more times. */
#define	PLUS		11		/* 	node	Match this (simple) thing 1 or more times. */
#define	WORDA		12		/* 	no		Match "" at wordchar, where prev is nonword */
#define	WORDZ		13		/* 	no		Match "" at nonwordchar, where prev is word */
#define	OPEN		20		/* 	no		Mark this point in input as start of #n. */
			/*	OPEN+1 is number 1, etc. */
#define	CLOSE		30		/* 	no		Analogous to OPEN. */

/*
 * Opcode notes:
 *
 * BRANCH	The set of branches constituting a single choice are hooked
 *		together with their "next" pointers, since precedence prevents
 *		anything being concatenated to any individual branch.  The
 *		"next" pointer of the last BRANCH in a choice points to the
 *		thing following the whole choice.  This is also where the
 *		final "next" pointer of each individual branch points; each
 *		branch starts with the operand node of a BRANCH node.
 *
 * BACK		Normal "next" pointers all implicitly point forward; BACK
 *		exists to make loop structures possible.
 *
 * STAR,PLUS	'?', and complex '*' and '+', are implemented as circular
 *		BRANCH structures using BACK.  Simple cases (one character
 *		per match) are implemented with STAR and PLUS for speed
 *		and to minimize recursive plunges.
 *
 * OPEN,CLOSE	...are numbered at compile time.
 */

/*
 * A node is one char of opcode followed by two chars of "next" pointer.
 * "Next" pointers are stored as two 8-bit pieces, high order first.  The
 * value is a positive offset from the opcode of the node containing it.
 * An operand, if any, simply follows the node.  (Note that much of the
 * code generation knows about this implicit relationship.)
 *
 * Using two bytes for the "next" pointer is vast overkill for most things,
 * but allows patterns to get big without disasters.
 */
#define	OP(p)	(*(p))
#define	NEXT(p)	( ( (*((p) + 1) & 0x0FF ) << 8 ) + ( *((p) + 2) & 0x0FF ) )
#define	OPERAND(p)	((p) + 3)



/*
 * Utility definitions.
 */
#ifndef CHARBITS
# define	UCHARAT(p)	(* (const uchar *)(p))
#else
# define	UCHARAT(p)	(*(p) & CHARBITS)
#endif

#define	ISMULT(c)	((c) == '*' || (c) == '+' || (c) == '?')

/*
 * Flags to be passed up and down.
 */
#define	HASWIDTH	01	/* Known never to match nil string. */
#define	SIMPLE		02	/* Simple enough to be STAR/PLUS operand. */
#define	SPSTART		04	/* Starts with * or +. */
#define	WORST		0	/* Worst case. */


/* **********************************************************************
*****	CHSRegExp - compile a regular expression into internal code	*****
*****																*****
*****	We can't allocate space until we know how big the compiled	*****
*****	form will be, but we can't compile it (and thus know how	*****
*****	big it is) until we've got a place to put the code.  So we	*****
*****	cheat:  we compile it twice, once with code generation		*****
*****	turned off and size counting turned on, and once "for real".*****
*****	This also means that we don't allocate space until we are	*****
*****	sure that the thing really will compile successfully, and	*****
*****	we never have to move the code and thus invalidate pointers	*****
*****	into it. (Note that it has to be in one piece because		*****
*****	delete[] must be able to free it all.)						*****
*****																*****
*****	Beware that the optimization-preparation code in here knows	*****
*****	about some of the structure of the compiled regexp.			*****
************************************************************************/
CHSRegExp::CHSRegExp( const char * theRegExp, tRegErrorProc proc /* =0 */ ) :
	mRegErrorProc( proc ),
	mRegmust(),
	mProgram(),
	mRegparse( theRegExp ),
	mRegnpar( 1 ),
	mRegcode( &mRegdummy ),
	mRegsize(),
	mReginput(),
	mRegbol( theRegExp ),
	mRegstartp( mStartPtrs ),
	mRegendp( mEndPtrs )
#ifdef DEBUG_REGEXP
	, mRegnarrate()
#endif
{
	// safety first
	if ( not theRegExp )
		regError( "CHSRegExp: nil argument" );
	
	// clear out the sub-pattern pointers
	for ( int i = 0; i < NSUBEXP; ++i )
		mStartPtrs[i] =	mEndPtrs[i] = nullptr;
	
	// First pass: determine size, legality.
#ifdef notdef
	if ( theRegExp[0] == '.' && theRegExp[1] == '*' )
		theRegExp += 2;  /* aid grep */
#endif
	
	regc( char(MAGIC) );
	
	int flags;
	if ( not reg( false, &flags ) )
		regError( "CHSRegExp: error during regular expression pre-compilation" );
	
	// Small enough for pointer-storage convention?
	if ( mRegsize >= 32767 )		/* Probably could be 65535 */
		regError( "CHSRegExp: regexp too large" );
	
	// Allocate space.
	mProgram = NEW_TAG("RegExp") char[ unsigned( mRegsize ) ];
	if ( not mProgram )
		regError( "CHSRegExp: out of memory" );
	
	// Second pass: emit code
	mRegparse = theRegExp;
	mRegnpar  = 1;
	mRegcode  = mProgram;
	regc( char(MAGIC) );
	if ( not reg( false, &flags ) )
		regError( "CHSRegExp: error during regular expression compilation" );
	
	// Dig out information for optimizations
	mRegstart = 0;						// Worst-case defaults.
	mReganch  = false;
	char * scan = mProgram + 1;			// First BRANCH.
	if ( END == OP(regnext(scan)) )		// Only one top-level choice.
		{
		scan = OPERAND( scan );
		
		// Starting-point info
		if ( OP(scan) == EXACTLY )
			mRegstart = *OPERAND( scan );
		else
		if ( OP(scan) == BOL )
			mReganch = true;
		
		/*
		 * If there's something expensive in the r.e., find the
		 * longest literal string that must appear and make it the
		 * mRegmust.  Resolve ties in favor of later strings, since
		 * the mRegstart check works with the beginning of the r.e.
		 * and avoiding duplication strengthens checking.  Not a
		 * strong reason, but sufficient in the absence of others.
		 */
		if ( flags & SPSTART )
			{
			const char * longest = nullptr;
			size_t len = 0;
			for ( ; scan; scan = regnext( scan ) )
				if ( OP(scan) == EXACTLY
				&&   std::strlen( OPERAND(scan) ) >= len )
					{
					longest = OPERAND( scan );
					len = std::strlen( longest );
					}
			mRegmust = longest;
			}
		}
}


/* **********************************************************************
*****																*****
************************************************************************/
CHSRegExp::~CHSRegExp()
{
	delete[] mProgram;
}


/* **********************************************************************
*****	reg - regular expression, i.e. main body or parenthesized	*****
*****	thing														*****
*****																*****
*****	Caller must absorb opening parenthesis.						*****
*****																*****
*****	Combining parenthesis handling with the base level of		*****
*****	regular expression is a trifle forced, but the need to tie	*****
*****	the tails of the branches to what follows makes it hard		*****
*****	to avoid.													*****
************************************************************************/
char *
CHSRegExp::reg( bool paren /* Parenthesized? */, int * flagp )
{
	char * ret = nullptr;
	int parno = 0;
	
	*flagp = HASWIDTH;	/* Tentatively. */
	
	/* Make an OPEN node, if parenthesized. */
	if ( paren )
		{
		if ( mRegnpar >= NSUBEXP )
			regError("reg: too many ()");
		parno = mRegnpar;
		++mRegnpar;
		ret = regnode( static_cast<char>( OPEN + parno ) );
		}
	
	/* Pick up the branches, linking them together. */
	int flags = 0;
	char * br = regbranch( &flags );
	if ( not br )
		return nullptr;
	if ( ret )
		regtail( ret, br );	/* OPEN -> first. */
	else
		ret = br;
	
	if ( not (flags & HASWIDTH) )
		*flagp &= ~HASWIDTH;
	*flagp |= flags & SPSTART;
	
	while ( '|' == *mRegparse || '\n' == *mRegparse )
		{
		++mRegparse;
		br = regbranch( &flags );
		if ( not br )
			return nullptr;
		regtail( ret, br );	/* BRANCH -> BRANCH. */
		
		if ( not (flags & HASWIDTH) )
			*flagp &= ~HASWIDTH;
		*flagp |= flags & SPSTART;
		}
	
	/* Make a closing node, and hook it on the end. */
	const char * ender = regnode( char( paren ? CLOSE + parno : END ) );
	regtail( ret, ender );
	
	/* Hook the tails of the branches to the closing node. */
	for ( br = ret; br; br = regnext( br ) )
		regoptail( br, ender );
	
	/* Check for proper termination. */
	if ( paren && *mRegparse++ != ')' )
		regError( "reg: unmatched ()" );
	else
	if ( not paren && *mRegparse != '\0' )
		{
		if ( ')' == *mRegparse )
			regError( "reg: unmatched ()" );
		else
			regError( "reg: junk on end of regular expression" );	// "Can't happen"
		/* NOTREACHED */
		}
	
	return ret;
}


/* **********************************************************************
*****	regbranch - one alternative of an | operator				*****
*****																*****
*****	Implements the concatenation operator.						*****
************************************************************************/
char *
CHSRegExp::regbranch( int * flagp )
{
	*flagp = WORST;		/* Tentatively. */
	
	char * ret = regnode( BRANCH );
	char * chain = nullptr;
	while ( * mRegparse != '\0' && * mRegparse != ')' &&
			* mRegparse != '\n' && * mRegparse != '|')
		{
		int flags;
		char * latest = regpiece( &flags );
		if ( not latest )
			return nullptr;
		*flagp |= flags & HASWIDTH;
		
		if ( not chain )			// First piece.
			*flagp |= (flags & SPSTART);
		else
			regtail( chain, latest );
		chain = latest;
		}
	
	if ( not chain )				// Loop ran zero times.
		regnode( NOTHING );
	
	return ret;
}


/* **********************************************************************
*****	regpiece - something followed by possible [*+?]				*****
*****																*****
*****	Note that the branching code sequences used for ? and the	*****
*****	general cases of * and + are somewhat optimized: they use	*****
*****	the same NOTHING node as both the endmarker for their		*****
*****	branch list and the body of the last branch.				*****
*****																*****
*****	It might seem that this node could be dispensed with		*****
*****	entirely, but the endmarker role is not redundant.			*****
************************************************************************/
char *
CHSRegExp::regpiece( int * flagp )
{
	int flags;
	char * ret = regatom( &flags );
	if ( not ret )
		return nullptr;
	
	char op = *mRegparse;
	if ( not ISMULT(op) )
		{
		*flagp = flags;
		return ret;
		}
	
	if ( not (flags & HASWIDTH) && op != '?' )
		regError( "regpiece: *+ operand could be empty" );
	
	* flagp = ( op != '+' ) ? (WORST | SPSTART) : (WORST | HASWIDTH);
	
	if ( '*' == op )
		{
		if ( flags & SIMPLE )
			reginsert( STAR, ret );
		else
			{
			/* Emit x* as (x&|), where & means "self". */
			reginsert( BRANCH, ret );			/* Either x */
			regoptail( ret, regnode(BACK) );	/* and loop */
			regoptail( ret, ret );				/* back */
			regtail( ret, regnode(BRANCH) );	/* or */
			regtail( ret, regnode(NOTHING) );	/* nil. */
			}
		}
	else
	if ( '+' == op )
		{
		if ( flags & SIMPLE )
			reginsert( PLUS, ret );
		else
			{
			/* Emit x+ as x(&|), where & means "self". */
			char * next = regnode( BRANCH );	/* Either */
			regtail( ret, next );
			regtail( regnode(BACK), ret );		/* loop back */
			regtail( next, regnode(BRANCH) );	/* or */
			regtail( ret, regnode(NOTHING) );	/* nil. */
			}
		}
	else
	if ( '?' == op )
		{
		/* Emit x? as (x|) */
		reginsert( BRANCH, ret );				/* Either x */
		regtail( ret, regnode(BRANCH) );		/* or */
		char * next = regnode( NOTHING );		/* nil. */
		regtail( ret, next );
		regoptail( ret, next );
		}
	
	++mRegparse;
	
	if ( ISMULT( *mRegparse ) )
		regError( "regpiece: nested *?+" );
	
	return ret;
}


/* **********************************************************************
*****	regatom - the lowest level									*****
*****																*****
*****	Optimization:  gobbles an entire sequence of ordinary		*****
*****	characters so that it can turn them into a single node,		*****
*****	which is smaller to store and faster to run.  Backslashed	*****
*****	characters are exceptions, each becoming a separate node;	*****
*****	the code is simpler that way and it's not worth fixing.		*****
************************************************************************/
char *
CHSRegExp::regatom( int * flagp )
{
	char * ret;
	int flags;
	
	*flagp = WORST;		/* Tentatively. */
	
	switch ( *mRegparse++ )
		{
		/* FIXME: these chars only have meaning at beg/end of pat? */
		case '^':
			ret = regnode( BOL );
			break;
		
		case '$':
			ret = regnode( EOL );
			break;
		
		case '.':
			ret = regnode( ANY );
			*flagp |= (HASWIDTH | SIMPLE);
			break;
		
		case '[':
			{
			if ( '^' == *mRegparse )		/* Complement of range. */
				{
				ret = regnode( ANYBUT );
				++mRegparse;
				}
			else
				ret = regnode( ANYOF );
			
			// these two lose their normal meanings if they're the first in the range
			if ( ']' == *mRegparse || '-' == *mRegparse )
				regc( *mRegparse++ );
			
			// accumulate the range
			while ( *mRegparse != '\0' && *mRegparse != ']' )
				{
				if ( '-' == *mRegparse )
					{
					// handle a subrange
					++mRegparse;
					
					// special cases for '-': it's a literal at the end of a class
					if ( ']' == *mRegparse || '\0' == *mRegparse )
						regc( '-' );
					else
						{
						uchar reClass = uchar( UCHARAT( mRegparse - 2 ) + 1 );
						uchar classend = UCHARAT( mRegparse );
						if ( reClass > classend + 1 )
							regError( "regatom: invalid [] range" );
						for ( ; reClass <= classend; ++reClass )
							regc( char( reClass ) );
						++mRegparse;
						}
					}
				else
					// normal character
					regc( *mRegparse++ );
				}
			
			// terminate the class-set
			regc( '\0' );
			
			if ( *mRegparse != ']' )
				regError( "regatom: unmatched []" );
			++mRegparse;
			*flagp |= (HASWIDTH | SIMPLE);
			}
			break;
		
		case '(':
			// recurse on a parenthesis
			ret = reg( true, &flags );
			if ( not ret )
				return nullptr;
			*flagp |= flags & (HASWIDTH | SPSTART);
			break;
		
		case '\0':
		case '|':
		case '\n':
		case ')':
			/* Supposed to be caught earlier. */
			regError( "regatom: internal error" );
			break;
		
		case '?':
		case '+':
		case '*':
			regError( "regatom: ?+* follows nothing" );
			break;
		
		case '\\':
			switch ( *mRegparse++ )
				{
				case '\0':
					regError( "regatom: \\ at the end of expression" );
					break;
				
				case '<':
					ret = regnode( WORDA );
					break;
				
				case '>':
					ret = regnode( WORDZ );
					break;
				
				/* FIXME: Someday handle \1, \2, ... */
				default:
					/* Handle general quoted chars in exact-match routine */
					goto de_fault;
				}
			break;
		
de_fault:
		default:
			/*
			 * Encode a string of characters to be matched exactly.
			 *
			 * This is a bit tricky due to quoted chars and due to
			 * '*', '+', and '?' taking the SINGLE char previous
			 * as their operand.
			 *
			 * On entry, the char at mRegparse[-1] is going to go
			 * into the string, no matter what it is.  (It could be
			 * following a \ if we are entered from the '\' case.)
			 *
			 * Basic idea is to pick up a good char in  ch  and
			 * examine the next char.  If it's *+? then we twiddle.
			 * If it's \ then we frozzle.  If it's other magic char
			 * we push  ch  and terminate the string.  If none of the
			 * above, we push  ch  on the string and go around again.
			 *
			 *  regprev  is used to remember where "the current char"
			 * starts in the string, if due to a *+? we need to back
			 * up and put the current char in a separate, 1-char, string.
			 * When  regprev  is nil,  ch  is the only char in the
			 * string; this is used in *+? handling, and in setting
			 * flags |= SIMPLE at the end.
			 */
			{
			const char * regprev = nullptr;
			
			--mRegparse;			/* Look at cur char */
			ret = regnode( EXACTLY );
			for (;;)
				{
				char ch = *mRegparse++;	/* Get current char */
				switch ( *mRegparse )	/* look at next one */
					{
					default:
						regc( ch );		/* Add cur to string */
						break;
					
					case '.': case '[': case '(':
					case ')': case '|': case '\n':
					case '$': case '^':
					case '\0':
					/* FIXME, $ and ^ should not always be magic */
			magic:
						regc( ch );	/* dump cur char */
						goto done;	/* and we are done */
					
					case '?':
					case '+':
					case '*':
						if ( not regprev ) 	/* If just ch in str, */
							goto magic;		/* use it */
						
						/* End multi-char string one early */
						mRegparse = regprev; /* Back up parse */
						goto done;
					
					case '\\':
						regc( ch );				/* Cur char OK */
						switch ( mRegparse[1] )	/* Look after \ */
							{
							case '\0':
							case '<':
							case '>':
							/* FIXME: Someday handle \1, \2, ... */
								goto done; /* Not quoted */
							
							default:
								/* Backup point is \, scan point is after it. */
								regprev = mRegparse;
								++mRegparse;
								continue;	/* NOT break; */
							}
					}
					regprev = mRegparse;	/* Set backup point */
				}
	done:
			regc( '\0' );
			*flagp |= HASWIDTH;
			if ( not regprev )		/* One char? */
				*flagp |= SIMPLE;
			}
			break;
		}
	
	return ret;
}


/* **********************************************************************
*****	regnode - emit a node										*****
************************************************************************/
char *			/* Location. */
CHSRegExp::regnode( char op )
{
	char * ret = mRegcode;
	if ( ret == &mRegdummy )
		{
		// just measuring, not compiling
		mRegsize += 3;
		return ret;
		}
	
	char * ptr = ret;
	*ptr++ = op;
	*ptr++ = '\0';		/* nil "next" pointer. */
	*ptr++ = '\0';
	mRegcode = ptr;
	
	return ret;
}


/* **********************************************************************
*****	regc - emit (if appropriate) a byte of code					*****
************************************************************************/
void
CHSRegExp::regc( char b )
{
	if ( mRegcode != &mRegdummy )
		*mRegcode++ = b;
	else
		++mRegsize;
}


/* **********************************************************************
*****	reginsert - insert an operator in front of already-emitted	*****
*****	operand														*****
*****																*****
*****	Means relocating the operand.								*****
************************************************************************/
void
CHSRegExp::reginsert( char op, char * opnd )
{
	if ( mRegcode == &mRegdummy )
		{
		// just measuring
		mRegsize += 3;
		return;
		}
	
	char * src = mRegcode;
	mRegcode += 3;
	char * dst = mRegcode;
	while ( src > opnd )
		*--dst = *--src;
	
	char * place = opnd;		/* Op node, where operand used to be. */
	*place++ = op;
	*place++ = '\0';
	*place++ = '\0';
}


/* **********************************************************************
*****	regtail - set the next-pointer at the end of a node chain	*****
************************************************************************/
void
CHSRegExp::regtail( char * p, const char * val )
{
	if ( p == &mRegdummy )
		return;
	
	/* Find last node. */
	char * scan = p;
	for (;;)
		{
		char * temp = regnext( scan );
		if ( not temp )
			break;
		scan = temp;
		}
	
	int offset;
	if ( OP(scan) == BACK )
		offset = int( scan - val );
	else
		offset = int( val - scan );
	
	scan[1] = char( (offset >> 8) & 0x0FF );
	scan[2] = char( offset & 0x0FF );
}


/* **********************************************************************
*****	regoptail - regtail on operand of first argument; nop		*****
*****	if operandless												*****
************************************************************************/
void
CHSRegExp::regoptail( char * p, const char * val )
{
	/* "Operandless" and "op != BRANCH" are synonymous in practice. */
	if ( not p || p == &mRegdummy || OP(p) != BRANCH )
		return;
	regtail( OPERAND(p), val );
}


/*
 * regexec and friends
 */

/* **********************************************************************
*****	regexec - match a regexp against a string					*****
************************************************************************/
bool
CHSRegExp::regexec( const char * theData )
{
	/* Be paranoid... */
	if ( not theData )
		regError( "regexec: input is NULL" );
		// NOTREACHED
	
	/* Check validity of mProgram. */
	if ( UCHARAT( mProgram ) != MAGIC )
		regError( "regexec: compiled regular expression is corrupted" );
		// NOTREACHED
	
	/* If there is a "must appear" string, look for it. */
	if ( mRegmust && not std::strstr( theData, mRegmust ) )
		return false;
	
	/* Mark beginning of line for ^ . */
	mRegbol = theData;
	
	/* Simplest case:  anchored match need be tried only once. */
	if ( mReganch )
		return regtry( theData );
	
	/* Messy cases:  unanchored match. */
	const char * s = theData;
	if ( mRegstart )
		{
		/* We know what char it must start with. */
		while ( (s = std::strchr( s, mRegstart )) != 0 )
			{
			if ( regtry(s) )
				return true;
			++s;
			}
		}
	else
		/* We don't -- general case. Try every single possibility. */
		do {
			if ( regtry(s) )
				return true;
			} while ( *s++ );
	
	/* Failure. */
	return false;
}


/* **********************************************************************
*****	regtry - try match at specific point						*****
************************************************************************/
bool
CHSRegExp::regtry( const char * string )
{
	mReginput  = string;
	mRegstartp = mStartPtrs;
	mRegendp   = mEndPtrs;
	
	const char ** sp = mStartPtrs;
	const char ** ep = mEndPtrs;
	for ( int i = NSUBEXP; i > 0; --i )
		{
		*sp++ = nullptr;
		*ep++ = nullptr;
		}
	
	if ( regmatch( mProgram + 1 ) )
		{
		mStartPtrs[0] = string;
		mEndPtrs[0]   = mReginput;
		return true;
		}
	
	return false;
}


/* **********************************************************************
*****	regmatch - main matching routine							*****
*****																*****
*****	Conceptually the strategy is simple:  check to see whether	*****
*****	the current node matches, call self recursively to see		*****
*****	whether the rest matches, and then act accordingly.			*****
*****	In practice we make some effort to avoid recursion, in		*****
*****	particular by going through "ordinary" nodes (that don't	*****
*****	need to know whether the rest of the match failed) by a		*****
*****	loop instead of by recursion.								*****
************************************************************************/
bool
CHSRegExp::regmatch( char * prog )
{
	char * scan = prog;		/* Current node. */
	char * next = nullptr;	/* Next node. */
	
#ifdef DEBUG_REGEXP
	if ( scan && mRegnarrate )
		fprintf( stderr, "%s(\n", regprop(scan) );
#endif
	
	while ( scan )
		{
#ifdef DEBUG_REGEXP
		if ( mRegnarrate )
			fprintf( stderr, "%s...\n", regprop(scan) );
#endif
		next = regnext( scan );
		
		switch ( OP(scan) )
			{
			case BOL:
				if ( mReginput != mRegbol )
					return false;
				break;
			
			case EOL:
				if ( *mReginput )
					return false;
				break;
			
			case WORDA:
				/* Must be looking at a letter, digit, or _ */
				if ( not std::isalnum( *mReginput )
				&&	 *mReginput != '_' )
					{
					return false;
					}
				
				/* Prev must be BOL or nonword */
				if ( mReginput > mRegbol
				&&	 ( std::isalnum( mReginput[-1] ) || mReginput[-1] == '_' ) )
					{
					return false;
					}
				break;
			
			case WORDZ:
				/* Must be looking at non letter, digit, or _ */
				if ( std::isalnum( *mReginput ) || *mReginput == '_' )
					return false;
				/* We don't care what the previous char was */
				break;
			
			case ANY:
				// only fail on NUL
				if ( '\0' == *mReginput )
					return false;
				++mReginput;
				break;
			
			case EXACTLY:
				{
				const char * opnd = OPERAND( scan );
				/* Inline the comparison of the first character, for speed. */
				if ( *opnd != *mReginput )
					return false;
				// first char matches so compare the rest all in one go.
				size_t len = std::strlen( opnd );
				if ( len > 1 && std::strncmp( opnd, mReginput, len ) != 0 )
					return false;
				mReginput += len;
				}
				break;
			
			case ANYOF:
	 			if ( '\0' == *mReginput
	 			||   nullptr == std::strchr( OPERAND(scan), *mReginput ) )
	 				{
					return false;
					}
				++mReginput;
				break;
			
			case ANYBUT:
	 			if ( '\0' == *mReginput
	 			||   nullptr != std::strchr( OPERAND(scan), *mReginput ) )
	 				{
					return false;
					}
				++mReginput;
				break;
			
			case NOTHING:
				break;
			case BACK:
				break;
			
			case OPEN+1:
			case OPEN+2:
			case OPEN+3:
			case OPEN+4:
			case OPEN+5:
			case OPEN+6:
			case OPEN+7:
			case OPEN+8:
			case OPEN+9:
				{
				int no = OP(scan) - OPEN;
				const char * save = mReginput;
				
				if ( regmatch( next ) )
					{
					/*
					 * Don't set mStartPtrs if some later
					 * invocation of the same parentheses
					 * already has.
					 */
					if ( not mRegstartp[ no ] )
						mRegstartp[ no ] = save;
					return true;
					}
				else
					return false;
				}
				break;
			
			case CLOSE+1:
			case CLOSE+2:
			case CLOSE+3:
			case CLOSE+4:
			case CLOSE+5:
			case CLOSE+6:
			case CLOSE+7:
			case CLOSE+8:
			case CLOSE+9:
				{
				int no = OP(scan) - CLOSE;
				const char * save = mReginput;
				
				if ( regmatch( next ) )
					{
					/*
					 * Don't set mEndPtrs if some later
					 * invocation of the same parentheses
					 * already has.
					 */
					if ( not mRegendp[ no ] )
						mRegendp[ no ] = save;
					return true;
					}
				else
					return false;
				}
				break;
			
			case BRANCH:
				{
				if ( OP(next) != BRANCH )		/* No choice. */
					next = OPERAND(scan);		/* Avoid recursion. */
				else
					{
					do {
						const char * save = mReginput;
						if ( regmatch( OPERAND(scan) ) )
							return true;
						mReginput = save;
						scan = regnext( scan );
						} while ( scan && OP(scan) == BRANCH );
					return false;
					/* NOTREACHED */
					}
				}
				break;
			
			case STAR:
			case PLUS:
				{
				/*
				 * Lookahead to avoid useless match attempts
				 * when we know what character comes next.
				 */
				char nextch = '\0';
				if ( OP(next) == EXACTLY )
					nextch = *OPERAND( next );
				int min = ( OP(scan) == STAR ) ? 0 : 1;
				const char * save = mReginput;
				int no = regrepeat( OPERAND(scan) );
				while ( no >= min )
					{
					/* If it could work, try it. */
					if ( '\0' == nextch || *mReginput == nextch )
						if ( regmatch( next ) )
							return true;
					/* Couldn't or didn't -- back up. */
					--no;
					mReginput = save + no;
					}
				return false;
				}
				break;
			
			case END:
				return true;	/* Success! */
				break;
			
			default:
				regError( "memory corruption" );
				// NOT REACHED
				break;
			}
		
		scan = next;
		}
	
	/*
	 * We get here only if there's trouble -- normally "case END" is
	 * the terminating point.
	 */
	regError( "regmatch: internal pointers are corrupted" );
	// NOT REACHED
	return false;
}


/* **********************************************************************
*****	regrepeat - repeatedly match something simple, report		*****
*****	how many													*****
************************************************************************/
int
CHSRegExp::regrepeat( const char * p )
{
	int count = 0;
	const char * scan = mReginput;
	const char * opnd = OPERAND( p );
	
	switch ( OP(p) )
		{
		case ANY:
			count = int( std::strlen( scan ) );
			scan += count;
			break;
		
		case EXACTLY:
			while ( *opnd == *scan )
				{
				++count;
				++scan;
				}
			break;
		
		case ANYOF:
			while ( *scan && std::strchr( opnd, *scan ) != nullptr )
				{
				++count;
				++scan;
				}
			break;
		
		case ANYBUT:
			while ( *scan && std::strchr( opnd, *scan ) == nullptr )
				{
				++count;
				++scan;
				}
			break;
		
		default:		/* Oh dear.  Called inappropriately. */
			regError( "regrepeat: Called inappropriately, internal foulup" );
			// NOT REACHED
			break;
		}
	mReginput = scan;
	
	return count;
}


/* **********************************************************************
*****	regnext - dig the "next" pointer out of a node				*****
************************************************************************/
char *
CHSRegExp::regnext( char * p ) const
{
	if ( p == &mRegdummy )
		return nullptr;
	
	int offset = NEXT(p);
	if ( 0 == offset )
		return nullptr;
	
	if ( OP(p) == BACK )
		return p - offset;
	else
		return p + offset;
}


#ifdef DEBUG_REGEXP

/* **********************************************************************
*****	regdump - dump a regexp onto stdout in vaguely				*****
*****	comprehensible form											*****
************************************************************************/
void
CHSRegExp::regdump( void ) const
{
	char op = EXACTLY;	/* Arbitrary non-END op. */
	char * s = mProgram + 1;
	
	while ( op != END )	/* While that wasn't END last time... */
		{
		op = OP( s );
		printf( "%2ld %s", s - mProgram, regprop(s) );	/* Where, what. */
		const char * next = regnext( s );
		if ( not next )		/* Next ptr. */
			printf( "(0)" );
		else
			printf( "(%ld)", next - mProgram );
		s += 3;
		if ( ANYOF == op || ANYBUT == op || EXACTLY == op )
			{
			/* Literal string, where present. */
			while ( *s )
				putchar( *s++ );
			++s;
			}
		putchar( '\n' );
		}
	
	/* Header fields of interest. */
	if ( mRegstart != '\0' )
		printf( "start `%c' ", mRegstart );
	if ( mReganch )
		printf( "anchored " );
	if ( mRegmust )
		printf( "must have \"%s\"", mRegmust );
	printf( "\n" );
}


/* **********************************************************************
*****	regprop - printable representation of opcode				*****
************************************************************************/
const char *
CHSRegExp::regprop( const char * op ) const
{
	const char * p;
	static char buf[ 32 ];
	
	strcpy( buf, ":" );
	
	switch ( OP(op) )
		{
		case BOL:		p = "BOL";		break;
		case EOL:		p = "EOL";		break;
		case ANY:		p = "ANY";		break;
		case ANYOF:		p = "ANYOF";	break;
		case ANYBUT:	p = "ANYBUT";	break;
		case BRANCH:	p = "BRANCH";	break;
		case EXACTLY:	p = "EXACTLY";	break;
		case NOTHING:	p = "NOTHING";	break;
		case BACK:		p = "BACK";		break;
		case END:		p = "END";		break;
		
		case OPEN+1 ... OPEN+9:
			snprintf( buf + std::strlen( buf ), sizeof buf, "OPEN%d", OP(op) - OPEN );
			p = nullptr;
			break;
		
		case CLOSE+1 ... CLOSE+9:
			snprintf( buf + std::strlen( buf ), sizeof buf, "CLOSE%d", OP(op) - CLOSE );
			p = nullptr;
			break;
		
		case STAR:		p = "STAR";		break;
		case PLUS:		p = "PLUS";		break;
		case WORDA:		p = "WORDA";	break;
		case WORDZ:		p = "WORDZ";	break;
		
		default:
			regError( "corrupted opcode" );
			// NOT REACHED
			break;
		}
	
	if ( p )
		strcat( buf, p );
	
	return buf;
}
#endif	// DEBUG_REGEXP


/* **********************************************************************
*****	regError() - report an error, throw an exception			*****
************************************************************************/
void
CHSRegExp::regError( const char * errorString ) const
{
	if ( mRegErrorProc )
		mRegErrorProc( errorString );
	
	throw paramErr;
}

