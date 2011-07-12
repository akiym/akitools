use strict;
use warnings;
use utf8;

package Amon2::Setup::Flavor::Teng;
use parent qw(Amon2::Setup::Flavor::Basic);

sub run {
    my $self = shift;

    $self->SUPER::run();

    $self->write_file('lib/<<PATH>>.pm', <<'...');
package <% $module %>;
use strict;
use warnings;
use parent qw/Amon2/;
our $VERSION='0.01';
use 5.008001;

__PACKAGE__->load_plugin(qw/DBI/);

use Teng;
use Teng::Schema::Loader;
sub db {
    my $self = shift;
    if (!defined $self->{db}) {
        my $dbh = $self->dbh;
        my $schema = Teng::Schema::Loader->load(
            dbh => $dbh,
            namespace => '<% $module %>::DB',
        );
        $self->{db} = Teng->new(
            dbh => $dbh,
            schema => $schema,
        );
    }
    return $self->{db};
}

1;
...

    $self->write_file('lib/<<PATH>>/DB.pm', <<'...');
package <% $module %>::DB;
use strict;
use warnings;
use parent qw/Teng/;

1;
...

    $self->write_file('lib/<<PATH>>/DB/Schema.pm', '');

    $self->write_file('script/make_schema.pl', <<'...');
use strict;
use warnings;
use File::Spec;
use File::Basename;
use lib File::Spec->catdir(dirname(__FILE__), '..', 'extlib', 'lib', 'perl5');
use lib File::Spec->catdir(dirname(__FILE__), '..', 'lib');
use <% $module %>;
use Teng::Schema::Dumper;

my $c = <% $module %>->bootstrap;
my $schema = Teng::Schema::Dumper->dump(
    dbh => $c->dbh,
    namespace => '<% $module %>::DB',
);

my $dest = File::Spec->catfile(dirname(__FILE__), '..', 'lib', '<% $module %>', 'DB', 'Schema.pm');
open my $fh, '>', $dest or die "Cannot open file: $dest: $!";
print {$fh} $schema;
close $fh;
...

    $self->write_file('Makefile.PL', <<'...');
use ExtUtils::MakeMaker;

WriteMakefile(
    NAME          => '<% $module %>',
    AUTHOR        => 'Some Person <person@example.com>',
    VERSION_FROM  => 'lib/<% $path %>.pm',
    PREREQ_PM     => {
        'Amon2'                           => '<% $amon2_version %>',
        'Amon2::DBI'                      => '0.05',
        'Text::Xslate'                    => '1.1005',
        'Text::Xslate::Bridge::TT2Like'   => '0.00008',
        'Plack::Middleware::ReverseProxy' => '0.09',
        'HTML::FillInForm::Lite'          => '1.09',
        'Time::Piece'                     => '1.20',
        'Teng'                            => '0.11',
    },
    MIN_PERL_VERSION => '5.008001',
    (-d 'xt' and $ENV{AUTOMATED_TESTING} || $ENV{RELEASE_TESTING}) ? (
        test => {
            TESTS => 't/*.t xt/*.t',
        },
    ) : (),
);
...
}

1;
