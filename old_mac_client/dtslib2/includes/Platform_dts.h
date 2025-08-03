/*
**	Platform_dts.h		dtslib2
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

#ifndef Platform_dts_h
#define Platform_dts_h

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif


#if 0
/*************************************************************************
this template hides the platform implementation details of an exposed class.

for example we want to implement widgets in a platform independent way.
widgets could be windows, menus, file specs, etc.
further suppose that the implementation can be determined at compile time.
see below for implementations that cannot be determined until run time.
we want to support the following semantics:

	widget w1;
	w1.DoSomething();
	widget w2 = w1;
	w2.DoSomething();
	w1 = w2;

the exposed interface file might be something like this:

-- widget-exposed.h --
class widget_private;
class widget
	{
public:
	DTSImplement<widget_private> pp;
	void DoSomething( char * str );
	};

the private include file might be something like this:

-- widget-private_mac.h --
class widget_private
	{
public:
	char buffer[256];
	};

the hidden platform implementation file might be something like this:

-- widget_mac.cp --
// this macro defines all of the gory details of tors and overloaded operators
DTSDefineImplementFirm(widget_private);
// implement the functions
void widget::DoSomething( char * str )
{
	strcpy( buffer, s );
	DebugStr( "\pDo Something!" );
}

the implementation is fairly simple.
the explanation is much more complex.
the public class new's a pointer to the private class.
however, c++ likes to assign pointers by copying them.
so we wrap the pointer in a class.
this class then handles all the gory c++ details.
like assignment copies the contents of the pointers instead of the pointers.
as do constructors with assignments.
we use a macro to define the constructors, destructor, and assignment operator
because the private class must be fully qualified -first-.

here's a similar example where the implementation is determined at runtime.

-- widget.h --
class widget_abstract;
class widget 
	{
public:
	DTSImplement<widget_abstract> pp;
	void DoSomething( char * s );
	};

-- widget_private.h --
class widget_abstract
	{
public:
	// implementation
	virtual ~widget_abstract( void ) {};
	static   widget_abstract * Factory( void );
	virtual  widget_abstract * Clone( void ) = 0;
	
	// interface
	virtual void DoSomething( char * s ) = 0;
	};
class widget_private_1 : public widget_abstract
	{
public:
	char buffer[256];
	virtual widget_abstract * Clone( void );
	virtual void DoSomething( char * s );
	};
class widget_private_2 : public widget_abstract
	{
public:
	char buffer[256];
	virtual widget_abstract * Clone( void );
	virtual void DoSomething( char * s );
	};

-- widget_mac.cp --
// this macro defines all of the gory details of tors and overloaded operators
DTSDefineImplementFlexible(widget_abstract)
// the widget functions all call through to the private virtual functions
void widget::DoSomething( char * s )
{
	if ( priv.p )
		priv.p->DoSomething(s);
}
// the factory decides which flavor to create
widget_abstract * widget_abstract::Factory()
{
	widget_abstract * w;
	if ( some_condition )
		w = NEW_TAG("widget_private_1") widget_private_1;
	else
		w = NEW_TAG("widget_private_2") widget_private_2;
	return w;
}
// the clone functions make copies of themselves
widget_abstract * widget_private_1::Clone()
{
	return NEW_TAG("widget_private_1") widget_private_1(*this);
}
widget_abstract * widget_private_2::Clone()
{
	return NEW_TAG("widget_private_2") widget_private_2(*this);
}
// implement the private functions
void widget_private_1::DoSomething( char * s )
{
	strcpy( buffer, s );
	DebugStr( "\pDo Something 1!" );
}
void widget_private_2::DoSomething( char * s )
{
	strcpy( buffer, s );
	DebugStr( "\pDo Something 2!" );
}

the difference between the fixed and the flexible methods:
flexible uses a virtual destructor and the copy operator deletes the old 
pointer and replaces it with a new pointer.
the latter is probably overkill.
but what the heck.
*/
#endif  // 0
/*************************************************************************/

// this template is used above the line, i.e. by the application
// as a base class for declaring the public widget class
template < class C >
class DTSImplement
{
public:
	C *		p;													// pointer to private widget class
					DTSImplement();									// widget w1;
					DTSImplement( const DTSImplement< C > & rhs );	// widget w2 = w1;
					~DTSImplement();
	DTSImplement &	operator =( const DTSImplement< C > & rhs );	// w1 = w2;
};

// This one is identical, except it doesn't require the implementation class
// to provide a copy constructor or an operator=(). We have a lot of classes
// like that. Note that the template declares both a copy-ctor and an assignment,
// as private members, but they're never defined. The compiler will only complain
// if you attempt to _use_ those functions.
template < class C >
class DTSImplementNoCopy
{
public:
	C *		p;													// pointer to private widget class
					DTSImplementNoCopy();
					~DTSImplementNoCopy();
private:
					DTSImplementNoCopy( const DTSImplementNoCopy< C > & );	// widget w2 = w1;
	DTSImplementNoCopy&
					operator=( const DTSImplementNoCopy< C > & );	// w1 = w2;
};


// this macro is used below the line, i.e. by the library,
// to define the private implementation of the gory c++ details.
// In this version, the implementation is entirely determined at compile time
#define DTSDefineImplementFirm( C )													\
	template<> DTSImplement< C >::DTSImplement() :									\
		p( NEW_TAG(#C) C ) {}														\
	template<> DTSImplement< C >::DTSImplement( const DTSImplement< C > & rhs ) :	\
		p( NEW_TAG(#C) C(*rhs.p) ) {}												\
	template<> DTSImplement< C >::~DTSImplement() { delete p; }						\
	template<> DTSImplement< C > &													\
	DTSImplement< C >::operator =( const DTSImplement< C > & rhs ) {				\
		if ( &rhs != this && p && rhs.p )											\
			*p = *rhs.p;															\
		return *this; }

// Ditto, for the no-copy variant of the idea
#define DTSDefineImplementFirmNoCopy( C )											\
	template<> DTSImplementNoCopy< C >::DTSImplementNoCopy() :						\
		p( NEW_TAG(#C) C ) {}														\
	template<> DTSImplementNoCopy< C >::~DTSImplementNoCopy() { delete p; }


// this macro is used below the line, i.e. by the library,
// to define the private implementation of the gory c++ details.
// In this version, the implementation is determined at run time
// via Clone() and Factory() methods
#define DTSDefineImplementFlexible( C )												\
	template<> DTSImplement< C >::DTSImplement() :									\
		p( C::Factory() ) {}														\
	template<> DTSImplement< C >::DTSImplement( const DTSImplement< C > & rhs ) :	\
		p( rhs.p ? rhs.p->Clone() : nullptr ) {}									\
	template<> DTSImplement< C >::~DTSImplement() { delete p; }						\
	template<>DTSImplement< C > &													\
	DTSImplement< C >::operator =( const DTSImplement< C > & rhs ) {				\
		if ( &rhs != this && rhs.p ) {												\
			delete p;																\
			p = rhs.p->Clone(); }													\
		return *this; }

// and re-ditto
#define DTSDefineImplementFlexibleNoCopy( C )										\
	template<> DTSImplementNoCopy< C >::DTSImplementNoCopy() :						\
		p( C::Factory() ) {}														\
	template<> DTSImplementNoCopy< C >::~DTSImplementNoCopy() { delete p; }

#endif	// Platform_dts_h

