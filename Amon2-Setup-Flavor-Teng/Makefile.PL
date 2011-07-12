use inc::Module::Install;
use Module::Install::AuthorTests;

name 'Amon2-Setup-Flavor-Teng';
all_from 'lib/Amon2/Setup/Flavor/Teng.pm';

requires(
    'parent' => 0,
);

tests 't/*.t';
test_requires 'Test::More' => 0.96; # done_testing, subtest
test_requires 'Test::Requires';
author_tests('xt');
WriteAll;
