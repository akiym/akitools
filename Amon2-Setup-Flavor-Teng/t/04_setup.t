use strict;
use warnings;
use utf8;
use Test::More;
use Test::Requires 'Amon2', 'File::Which';
use File::Temp qw/tempdir/;
use FindBin;
use File::Spec;
use lib File::Spec->catfile($FindBin::Bin, '..', 'lib');
use Plack::Util;
use Plack::Test;
use Cwd;
use Test::More;
use App::Prove;
use File::Which 'which';
use Config;

&main; done_testing; exit;

sub main {
    my $old_cwd = Cwd::cwd;
        &main_test;
    chdir $old_cwd;
}

sub main_test {
    my $dir = tempdir(CLEANUP => 1);
    chdir $dir or die $!;
    unshift @INC, File::Spec->catfile($dir, 'Hello', 'lib');

    my $setup = which('amon2-setup.pl');
    my $libdir = File::Spec->catfile($FindBin::Bin, '..', 'lib');
    !system $^X, '-I', $libdir, $setup, '--flavor=Basic', '--flavor=Teng', 'Hello' or die $!;
    chdir 'Hello' or die $!;

    note '-- run prove';
    system "$^X Makefile.PL";
    system $Config{make};
    my $app = App::Prove->new();
    $app->process_args('-Ilib', '-I'.File::Spec->catfile($FindBin::Bin, '..', '..', 'lib'), <t/*.t>, <xt/*.t>);
    ok($app->run);
}
