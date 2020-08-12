RELAY
=====

  transparent relay
  -----------------

    {"mode":"relay", "args": {"listen":"1.1.1.1:1111", "target":"2.2.2.2:2222"}}

      client ------TCP------> [relay ------TCP------> server
                           listen target

  encrypted relay
  ---------------

    {"mode":"inner", "args": {"listen":"1.1.1.1:1111", "target":"2.2.2.2:2222", "secret":"0123456789abcdef"}}
    {"mode":"outer", "args": {"listen":"2.2.2.2:2222", "target":"3.3.3.3:3333", "secret":"0123456789abcdef"}}

      client ------TCP------> [inner ======TCPS======> [outer ------TCP------> server
                           listen target            listen target


REVERSE RELAY
=============

  reverse transparent relay
  -------------------------

    {"mode":"finder", "args": {"listen":"1.1.1.1:1111", "target":"2.2.2.2:2222", "secret":"123456"}}
    {"mode":"mapper", "args": {"listen":"2.2.2.2:2222", "target":"3.3.3.3:3333", "secret":"123456"}}

      server <------TCP------ finder <------TCP------>> [mapper] <------TCP------ client
                          listen  target             listen  target

  reverse encrypted relay
  -----------------------

    {"mode":"broker", "args": {"listen":"1.1.1.1:1111", "target":"2.2.2.2:2222", "secret":"0123456789abcdef"}}
    {"mode":"router", "args": {"listen":"2.2.2.2:2222", "target":"3.3.3.3:3333", "secret":"0123456789abcdef"}}

      server <------TCP------ broker <======TCPS======>> [router] <------TCP------ client
                          listen  target              listen  target


PROXY
=====

  transparent proxy
  -----------------

    {"mode":"http", "args": {"listen":"1.1.1.1:8080"}}

      client ------TCP------> [http ~~~~~~TCP~~~~~~> server
                           listen

    {"mode":"sock", "args": {"listen":"1.1.1.1:1080"}}

      client ------TCP------> [sock ~~~~~~TCP~~~~~~> server
                           listen

  encrypted proxy
  ---------------

    {"mode":"https", "args": {"listen":"1.1.1.1:8080", "target":"2.2.2.2:2222", "secret":"0123456789abcdef"}}
    {"mode":"socks", "args": {"listen":"1.1.1.1:1080", "target":"2.2.2.2:2222", "secret":"0123456789abcdef"}}
    {"mode":"agent", "args": {"listen":"2.2.2.2:2222", "secret":"0123456789abcdef"}}

      client ------TCP------> [https ======TCPS======> [agent ~~~~~~TCP~~~~~~> server
                           listen target            listen

      client ------TCP------> [socks ======TCPS======> [agent ~~~~~~TCP~~~~~~> server
                           listen target            listen

